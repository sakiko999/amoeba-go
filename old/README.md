# Electron 多 tab 架构 demo

bun workspaces monorepo；**每个 tab = 一个页面组件包**（`packages/tab-*`），运行时每个 tab 实例 = 主进程管理的独立 `WebContentsView`（独立 webContents / preload / storage / 生命周期）。electron-vite 6 构建（dev 有 HMR + 主进程自动重启），TypeScript 7（原生编译器）typecheck，`bun fmt` + `biome lint`（bun 1.4 没有内置 lint 子命令，用 biome 补位）。

## 架构

```
apps/desktop ──────────────────────────────────────────────┐
│  main (node)                                             │
│   ├─ tabs.ts          TabManager：页面组件 tab 实例        │
│   │                   （WebContentsView 增删/布局/菜单）    │
│   ├─ pages/           PageRegistry：扫描/安装/卸载/重载页面 │
│   │                   page:// 协议 + main 扩展生命周期     │
│   └─ index.ts         窗口 + IPC + --smoke 冒烟           │
│  preload (sandbox)    按角色注入 window.host / window.tab │
│  ui (React)           tab 条 + 加载过渡页（Tailwind）      │
└────────────┬────────────────────────────▲────────────────┘
             │ IPC（tabs:changed 状态推送）                │
             ▼                            │
packages/tab-clock / tab-notes / tab-sysinfo
   页面组件（component.json + index.html + renderer.ts）
   各自 electron-vite 构建到 out/page/，由 pages:prepare 投影

apps/desktop/out/pages/<id>/    组件包投影（构建产物，gitignored）
   component.json + renderer/ + 可选 preload/ + 可选 main.js（CJS）
   page://<id>/<entry>，每页面 persist:page-<id> 分区

packages/host-sdk        @host/sdk：宿主契约（tab / runtime 页面 / HostState 渠道）
packages/page-vendor    renderer 共享层构建（vendor:// + import map，非组件）
packages/ui             @demo/ui：全仓公共 shadcn 组件库（CLI 全量生成，61 组件 + 主题 CSS）
packages/media-manager  submodule：媒体库管理器（Electron 前端 + 后端；刮削/NFO/整理）
                        也可打包为 runtime 页面包（bun run pages:prepare）
```

要点：

- 所有 tab 都是页面组件（`packages/tab-*` 也走 `component.json` + `out/page`）；宿主没有编译期静态 tab。默认打开 home，home 不可用时回退第一个可用页面组件。
- tab 页面与 host 页面互不知道对方，一切状态经主进程（`TabManager.broadcast()` 单向投影给 tab 条）。
- tab 内唯一能力是 preload 暴露的 `window.tab`（setTitle / close / ping / systemInfo）。
- `contextIsolation` + `sandbox` 全开，只加载本地文件；tab 侧 IPC 一律用 sender 定位调用者，不信任页面自报身份。
- 安全默认：拦截一切导航与 `window.open`（主进程 `hardenWebContents`）、权限请求默认全拒、preload 未知角色不注入任何桥。
- 每个 tab 实例独立 webContents；页面组件用 `persist:page-<id>` 分区（同一组件跨实例/重启保留，按 instanceId 区分的数据仍相互隔离）。
- tab 条为 Chrome 风格 44px（视觉移植自 [cherry-studio](tmp/cherry-studio) 的 `AppShellTabBar`：状态色 color-mix 公式、标题渐隐、hover 展开的 X、关闭 collapse 动画、中键/双击关闭）；cherry-studio 本体是单 renderer 内 React tab（TabsProvider + 每 tab 独立 MemoryRouter + `<Activity>` KeepAlive），与本项目“每 tab 一个 WebContentsView”是两种架构，这里只抄了 UI 层。
- 无系统标题栏：tab 条即标题栏（拖拽区），右侧窗口按钮为原生 `titleBarOverlay`（cherry Windows 同款，高 44 随主题变色）；tab 条尾部有亮/暗切换按钮，主题由主进程 `HostStateStore` 唯一持有，经 `host:state` 广播到 host 与所有 tab（`theme:changed` 保留给旧页面）。
- “+” 新建与 tab 右键菜单（关闭/关闭其他/关闭右侧）由主进程 `Menu.popup()` 弹出：host 页面 DOM 会被 tab 的原生子视图遮挡，DOM 菜单不可行。
- renderer 加载：dev 走 electron-vite dev server（`ELECTRON_RENDERER_URL`），生产走 `app://` 自定义协议映射 `out/renderer`——ESM 模块脚本需要 http 型 origin，file:// 不行；协议须按 session 注册（每个 tab 的独立 partition 是另一个 session）。
- 调试入口：tab 条的 bug 按钮弹出原生菜单——调试主进程（带 `--inspect=9229` 重启，`chrome://inspect` 连接）、调试启动器界面（host 页面 DevTools）、调试内容页面（当前 tab DevTools）；F12 仍可开关当前焦点 webContents 的 devtools。

### 宿主状态（HostState）

- `apps/desktop/src/main/state.ts` 的 `HostStateStore` 是唯一状态源：`get / set / subscribe`，
  持久化到 `userData/host-state.json`（坏文件/非法值回退默认 dark）。新增全局状态只需扩展
  `HostState` 与 `set()` 校验，广播层自动覆盖。
- 状态变更 → 更新 `titleBarOverlay`，并向 host 与所有 tab 广播 `host:state`；
  同时发 `theme:changed` 兼容旧页面。
- 页面接入（preload）：`sendSync("host:state:sync")` 首帧同步取初始状态，`on("host:state")`
  订阅变更；渠道常量与类型来自 `packages/host-sdk`（`HOST_STATE_CHANNELS` / `HostState`）。
  media-manager 的 preload 已注入 `window.hostState`，renderer 在首帧前切换 `.dark` 类。
- 页面 tab 用 `window.tab.state()/onStateChanged()` 订阅整份状态（pageBridge 注入）；
  `theme()/onThemeChanged` 保留兼容（内部仍是同一份状态）。
- smoke 会真实切换一次宿主主题并断言 media 页面跟随（`SMOKE: theme propagated`）。

### renderer 共享层（vendor:// + import maps）

页面组件与宿主 UI 共用一份预构建的 `react / react-dom / @demo/ui`（`packages/page-vendor`）：

- `pages:prepare` 先构建共享层：Bun.build 逐 entry 产出内容 hash 文件 + `manifest.json`
  （文件名与版本）。
- 消费方 electron-vite 构建读 manifest，把共享裸名 external，并在 `transformIndexHtml`
  里把 import map 插在 **CSP meta 之后**（必须早于 module script），同时给 CSP 追加其
  `'sha256-…'`；源 HTML 不含任何占位脚本。
- 宿主 `vendor://` 协议提供 `out/vendor`（带 `Access-Control-Allow-Origin: *`）；
  dev / 独立构建不 external，回退全内联。
- 实测页面 JS：home ~19KB、inspector ~26KB、media-manager ~57KB，vendor 全宿主一份，
  并以 120KB/页的体积守卫防 external 静默失效。CSS 暂不共享。

设计取舍、扩展共享集的步骤与常见坑见 [`docs/renderer-shared-layer.md`](docs/renderer-shared-layer.md)。

### 模块格式（CJS / ESM）

- **renderer**：原生 ESM（页面与宿主 UI 都是 ESM bundle）。
- **main process**：Electron ≥28 支持 ESM；本项目由 electron-vite 输出 CJS 入口。
- **preload**：沙箱 preload（`sandbox: true`）只能 CJS；本项目的 preload 全部沙箱化。
- **组件 `main.js`**：必须是 CJS——宿主用 `createRequire` 加载并删除 `require.cache`
  实现热重载。这是本项目加载器的设计选择，**不是 Electron 只支持 CJS**；ESM 没有可清除的
  模块缓存 API，无法可靠卸载传递依赖。

### Runtime 页面（热插拔）

- 页面包 = `page.json` + `renderer/` + `preload/` + 可选 `main.js`（CJS 主进程扩展）。
  内置页面从 `apps/desktop/out/pages/` 扫描，外部页面安装到 `userData/pages/<id>/`。
- `page://<id>/<entry>` 协议按 session 注册，路径用 `relative + realpath` 双重校验；
  页面 view 沿用 sandbox / contextIsolation / 权限全拒 / 导航拦截。
- `+` 菜单底部：安装页面…（选目录 + 信任警告 + 复制）、管理页面（重载 / 卸载）；
  页面 tab 右键也可重载/卸载。安装、卸载、重载均无需重启宿主（安装/卸载内置 tab 期构建的
  tab 不受影响）。
- 页面 main 扩展通过 `createRequire` 加载，重载时清 `require.cache`；宿主包装并跟踪
  `ipc.handle/on`，卸载时精确移除。页面可声明并发使用宿主窗口（`ctx.getWindow()`）。
- 页面 tab 使用 `persist:page-<id>` 分区：localStorage 跨 tab 关闭/宿主重启保留
  （media-manager 的媒体库根因此不会丢）。
- 宿主通过 `session.registerPreloadScript` 叠加 `pageBridge`：页面即使自带 preload，也能拿到
  `window.tab`（instanceId/kind/setTitle/close/ping/systemInfo/state/onStateChanged）与
  `window.hostState`（initial + onChanged）。页面自有 preload 不要再注入这两个同名全局。
- 启动顺序：先建窗口并注册 session 策略，再后台激活页面 main（每页 8s 超时）；main 激活失败
  时 renderer 仍可打开，菜单与安装/重载对话框会显示错误原因。
- 安全边界：页面的 `main.js` 等于完整主进程权限，安装时强制警告（只装可信来源）；卸载只做
  运行时注销、不删除用户目录；重启后会重新扫描到该页面。

## 命令

| 命令                                        | 作用                                                     |
| ------------------------------------------- | -------------------------------------------------------- |
| `bun install`                               | 安装（bun workspaces）                                   |
| `bun run dev`                               | electron-vite dev（HMR、主进程自动重启、inspector 9229） |
| `bun run build`                             | electron-vite build → `apps/desktop/out/`                |
| `bun run preview`                           | build 后直接以生产模式启动（无 HMR，启动约 1s）          |
| `bun run desktop:dist`                      | 宿主独立分发（@electron/packager asar → release/）       |
| `bun run smoke`                             | 无头冒烟：所有已注册页面组件加载完则退出码 0             |
| `bun run pages:prepare`                     | 打包全部组件并同步到 apps/desktop/out/pages              |
| `bun run media`                             | media-manager dev（独立 Electron 应用，HMR）             |
| `bun run media:debug`                       | 同上，另开主进程 inspector 9230 + sourcemap              |
| `bun run media:smoke`                       | media-manager 无头冒烟                                   |
| `bun run --cwd packages/media-manager dist` | media-manager 独立分发（asar，产物在 release/）          |
| `bun run typecheck`                         | `tsc --noEmit`（TypeScript 7 原生）                      |
| `bun run lint`                              | `biome lint .`                                           |
| `bun run fmt`                               | `bun fmt .`                                              |

> dev 只做组件投影、不自动打包：首次运行或改完页面组件（含 `tab-*`）源码后，先
> `bun run pages:prepare`（`build` / `smoke` 已包含该步）。

### 生产产物

| 形态                   | 命令                                        | 产物                                                           |
| ---------------------- | ------------------------------------------- | -------------------------------------------------------------- |
| 宿主（asar 分发）      | `bun run desktop:dist`                      | `apps/desktop/release/Tabs Demo-win32-x64/Tabs Demo.exe`       |
| 宿主（本地运行）       | `bun run preview`                           | `apps/desktop/out/`                                            |
| media-manager 独立应用 | `bun run --cwd packages/media-manager dist` | `packages/media-manager/release/Media Manager-win32-x64/`      |
| media-manager 页面包   | `bun run pages:prepare`                     | `packages/media-manager/out/page/` → `apps/desktop/out/pages/` |

- 产物运行时零依赖：main 侧代码由 electron-vite 打进 `out/main`。因此宿主侧的 workspace 包
  （如 `@host/sdk`）必须放 `devDependencies`——`dependencies` 会被 electron-vite externalize，
  而 asar 内没有 node_modules。页面组件则在各自包内构建成自包含产物。
- `app://` / `page://` 一律用 Node `fs.readFile` 读文件（`net.fetch(file://)` 读不到 asar）。
- 验收：`"Tabs Demo.exe" --smoke` 退出码须为 0。GUI 应用没有 stdout，smoke 日志写在
  `%TEMP%/tabs-smoke-<pid>.log`（或由 `TABS_SMOKE_LOG` 指定）。

F12 可开关当前 webContents（host 或任意 tab）的 devtools；tab 条右侧 bug 按钮提供统一的调试菜单（主进程 / 启动器界面 / 内容页）。

### VS Code 调试

仓库自带 `.vscode/launch.json` + `tasks.json`：两个主进程配置的 `preLaunchTask` 会自动启动
对应 dev 进程（后台任务，等到就绪日志出现再 attach），按 F5 即可。
attach 都不设 `restart`：关闭应用窗口 → Electron 退出 → electron-vite 随之退出 → 调试会话与 dev
任务自动结束；代价是改主进程触发热重启后需要重新 F5（dev 任务若已结束会自动重启）。

| 配置                         | 自动启动的 task | 说明                                                                                        |
| ---------------------------- | --------------- | ------------------------------------------------------------------------------------------- |
| `Desktop: Main (9229)`       | `dev: desktop`  | 宿主主进程；`sourceMaps` + `outFiles` 已配，可直接在 `apps/desktop/src/main/**/*.ts` 下断点 |
| `Media Manager: Main (9230)` | `dev: media`    | media-manager 独立应用主进程，源码映射到 `packages/media-manager/src/main/**/*.ts`          |

renderer 不配 VS Code 入口：宿主 dev 已默认带 `--inspect=9229 --remoteDebuggingPort=9333
--sourcemap`，需要时用 Chrome/Edge 打开 `chrome://inspect` 连 `localhost:9333`，或直接 F12 /
调试菜单开 devtools；runtime 页面（`page://`）的 main 扩展挂在宿主主进程的 attach 会话里。

生产构建不带 inspector；无 inspector 时 bug 菜单的「调试主进程」会带 `--inspect=9229` 重启。

## 新增一个页面组件（含 tab 页面，无需改宿主）

1. 复制 `packages/tab-clock` → `packages/tab-foo`：改 `package.json` 的 name，改
   `component.json` 的 `id` / `name` / `color`（id 必须是小写 DNS label，也是 `page://` host，
   目录名建议 `tab-<id>`）。
2. 入口仍是 `src/index.html` + `renderer.ts`，`scripts/pack.ts` 会构建到 `out/page/`。
3. 在根 `scripts/pack-components.ts` 的 `baseComponents` 加一行 `["tab-foo", "pack"]`，
   然后 `bun run pages:prepare`。
4. 宿主启动/刷新后自动出现在 `+` 菜单与首页（`PageRegistry` 扫描 `out/pages`），
   不需要改宿主代码。

## 新增一个 runtime 页面（无需改宿主）

页面包目录结构：

```
my-page/
├── page.json          { id, name, color?, entry, preload?, main? }
├── index.html         入口（entry）
├── preload.js         sandbox CJS preload（可选）
└── main.js            可选主进程扩展（CJS，导出 activate/deactivate）
```

`main.js`：

```js
exports.activate = (ctx) => {
  ctx.ipc.handle("my:ping", () => "pong");
  // ctx.pageRoot / ctx.dataDir / ctx.manifest / ctx.getWindow() / ctx.log()
};
exports.deactivate = () => {
  /* 清理 IPC 之外的资源；IPC 由宿主兜底移除 */
};
```

安装方式：把目录复制到 `apps/desktop/out/pages/<id>/`（内置）或运行宿主后从 `+` 菜单
「安装页面…」选择目录（外部，会弹出主进程权限警告）。页面 main 只接受 CJS；页面 id 必须
是小写 DNS label。契约类型见 `packages/host-sdk`。

## 已知简化（升级路径）

- tab 条未抄 cherry-studio 的 pin 区、拖拽排序、close-in-place 宽度冻结、tab 间 hairline divider（升级路径：useTabDrag + 冻结/thaw 状态机 + 独立 divider 元素）。
- 主题仅亮/暗两态（cherry 有跟随系统）；状态已持久化在 `userData/host-state.json`。
- `onTabsChanged` 无 unsubscribe、whoami 用一次 `sendSync`（页面级生命周期，demo 取舍；host 重载靠 did-finish-load 重广播补齐）。
- 关闭全部 tab 后是空态，不自动重建首页。
- tsconfig 是单一 program（main/renderer 共用 DOM + node 类型），类型层不区分运行环境 → 升级路径：拆 project references。
- Notes 草稿存在 tab 实例的 sessionStorage 里，随 tab 关闭即清（跨会话持久化应改由主进程存）。
