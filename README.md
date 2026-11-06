# Electron 多 tab 架构 demo

bun workspaces monorepo；**每个 tab = 一个子包**，运行时每个 tab 实例 = 主进程管理的独立 `WebContentsView`（独立 webContents / preload / storage / 生命周期）。electron-vite 6 构建（dev 有 HMR + 主进程自动重启），TypeScript 7（原生编译器）typecheck，`bun fmt` + `biome lint`（bun 1.4 没有内置 lint 子命令，用 biome 补位）。

## 架构

```
apps/desktop ──────────────────────────────────────────────┐
│  main (node)                                             │
│   ├─ registry.ts      静态 tab 清单（编译期）              │
│   ├─ tabs.ts          TabManager：静态 tab + runtime 页面  │
│   │                   （WebContentsView 增删/布局/菜单）    │
│   ├─ pages/           PageRegistry：扫描/安装/卸载/重载页面 │
│   │                   page:// 协议 + main 扩展生命周期     │
│   └─ index.ts         窗口 + IPC + --smoke 冒烟           │
│  preload (sandbox)    按角色注入 window.host / window.tab │
│  ui (browser)         tab 条 + “+” 菜单（host 页面）       │
└────────────┬────────────────────────────▲────────────────┘
             │ workspace:*                │ IPC（tabs:changed 状态推送）
             ▼                            │
packages/tab-clock / tab-notes / tab-sysinfo
   manifest（id/title/color）+ index.html + renderer.ts
   经 electron-vite 多页构建到 out/renderer/（产物路径 = 源路径相对仓库根）

apps/desktop/pages/<id>/    runtime 页面包（构建产物，gitignored）
   page.json + renderer/ + preload/ + 可选 main.js（CJS）
   page://<id>/<entry>，每页面 persist:page-<id> 分区

packages/page-sdk       @pages/sdk：页面包契约（PageManifest / main 扩展上下文）
packages/ui             @demo/ui：全仓公共 shadcn 组件库（CLI 全量生成，61 组件 + 主题 CSS）
packages/media-manager  submodule：媒体库管理器（Electron 前端 + 后端；刮削/NFO/整理）
                        也可打包为 runtime 页面包（bun run pages:prepare）
```

要点：

- tab 页面与 host 页面互不知道对方，一切状态经主进程（`TabManager.broadcast()` 单向投影给 tab 条）。
- tab 内唯一能力是 preload 暴露的 `window.tab`（setTitle / close / ping / systemInfo）。
- `contextIsolation` + `sandbox` 全开，只加载本地文件；tab 侧 IPC 一律用 sender 定位调用者，不信任页面自报身份。
- 安全默认：拦截一切导航与 `window.open`（主进程 `hardenWebContents`）、权限请求默认全拒、preload 未知角色不注入任何桥。
- 每个 tab 实例独立 webContents + 独立内存 partition（storage 互不可见，关闭即清）；多开同一个 tab kind 互不串数据。
- tab 条为 Chrome 风格 44px（视觉移植自 [cherry-studio](tmp/cherry-studio) 的 `AppShellTabBar`：状态色 color-mix 公式、标题渐隐、hover 展开的 X、关闭 collapse 动画、中键/双击关闭）；cherry-studio 本体是单 renderer 内 React tab（TabsProvider + 每 tab 独立 MemoryRouter + `<Activity>` KeepAlive），与本项目“每 tab 一个 WebContentsView”是两种架构，这里只抄了 UI 层。
- 无系统标题栏：tab 条即标题栏（拖拽区），右侧窗口按钮为原生 `titleBarOverlay`（cherry Windows 同款，高 44 随主题变色）；tab 条尾部有亮/暗切换按钮，主题由主进程 `HostStateStore` 唯一持有，经 `host:state` 广播到 host、静态 tab 与 runtime 页面（`theme:changed` 保留给旧 tab）。
- “+” 新建与 tab 右键菜单（关闭/关闭其他/关闭右侧）由主进程 `Menu.popup()` 弹出：host 页面 DOM 会被 tab 的原生子视图遮挡，DOM 菜单不可行。
- renderer 加载：dev 走 electron-vite dev server（`ELECTRON_RENDERER_URL`），生产走 `app://` 自定义协议映射 `out/renderer`——ESM 模块脚本需要 http 型 origin，file:// 不行；协议须按 session 注册（每个 tab 的独立 partition 是另一个 session）。
- 调试入口：tab 条的 bug 按钮弹出原生菜单——调试主进程（带 `--inspect=9229` 重启，`chrome://inspect` 连接）、调试启动器界面（host 页面 DevTools）、调试内容页面（当前 tab DevTools）；F12 仍可开关当前焦点 webContents 的 devtools。

### 宿主状态（HostState）

- `apps/desktop/src/main/state.ts` 的 `HostStateStore` 是唯一状态源：`get / set / subscribe`，
  持久化到 `userData/host-state.json`（坏文件/非法值回退默认 dark）。新增全局状态只需扩展
  `HostState` 与 `set()` 校验，广播层自动覆盖。
- 状态变更 → 更新 `titleBarOverlay`，并向 host、所有静态 tab、所有 runtime 页面广播
  `host:state`；同时发 `theme:changed` 兼容旧 tab。
- 页面接入（preload）：`sendSync("host:state:sync")` 首帧同步取初始状态，`on("host:state")`
  订阅变更；渠道常量与类型来自 `packages/page-sdk`（`HOST_STATE_CHANNELS` / `HostState`）。
  media-manager 的 preload 已注入 `window.hostState`，renderer 在首帧前切换 `.dark` 类。
- 静态 tab 用 `window.tab.state()/onStateChanged()` 订阅整份状态；`theme()/onThemeChanged`
  保留兼容（内部仍是同一份状态）。
- smoke 会真实切换一次宿主主题并断言 media 页面跟随（`SMOKE: theme propagated`）。

### Runtime 页面（热插拔）

- 页面包 = `page.json` + `renderer/` + `preload/` + 可选 `main.js`（CJS 主进程扩展）。
  内置页面从 `apps/desktop/pages/` 扫描，外部页面安装到 `userData/pages/<id>/`。
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
| `bun run smoke`                             | 无头冒烟：静态 tab + 已注册页面全部加载完则退出码 0      |
| `bun run pages:prepare`                     | 构建 media-manager 页面包并同步到 apps/desktop/pages     |
| `bun run media`                             | media-manager dev（独立 Electron 应用，HMR）             |
| `bun run media:smoke`                       | media-manager 无头冒烟                                   |
| `bun run --cwd packages/media-manager dist` | media-manager 独立分发（asar，产物在 release/）          |
| `bun run typecheck`                         | `tsc --noEmit`（TypeScript 7 原生）                      |
| `bun run lint`                              | `biome lint .`                                           |
| `bun run fmt`                               | `bun fmt .`                                              |

F12 可开关当前 webContents（host 或任意 tab）的 devtools；tab 条右侧 bug 按钮提供统一的调试菜单（主进程 / 启动器界面 / 内容页）。

主进程调试：`bun run dev` 默认以 `--inspect=9229` 启动主进程（修改主进程文件时 electron-vite
自动重启，inspector 仍在）。用 Chrome/Edge 打开 `chrome://inspect` 添加 `localhost:9229`，
或用 VS Code attach：

```jsonc
{ "type": "node", "request": "attach", "name": "Attach Main", "port": 9229 }
```

生产构建不带 inspector；无 inspector 时 bug 菜单的「调试主进程」会带 `--inspect=9229` 重启。

## 新增一个 tab（编译期）

1. 复制 `packages/tab-clock` → `packages/tab-foo`：改 `package.json` 的 name，改 manifest 的 `id`（目录名必须是 `tab-<id>`，与 id 一致）。
2. 在 `apps/desktop` 里 `bun add @tabs/foo`（workspace 链接）。
3. `apps/desktop/src/main/registry.ts` 加一行 import + 数组项。
4. `apps/desktop/electron.vite.config.ts` 的 renderer input 加 `"tabs/foo/index": resolve(repoRoot, "packages/tab-foo/src/index.html")`。

第 4 步目前仍是手写（构建期 MPA 入口无法从 registry.ts 自动推导）；漏掉时 dev 正常、生产构建
不会产出该 tab 的 HTML。

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

安装方式：把目录复制到 `apps/desktop/pages/<id>/`（内置）或运行宿主后从 `+` 菜单
「安装页面…」选择目录（外部，会弹出主进程权限警告）。页面 main 只接受 CJS；页面 id 必须
是小写 DNS label 且不能与静态 tab id 冲突。契约类型见 `packages/page-sdk`。

## 已知简化（升级路径）

- tab 条未抄 cherry-studio 的 pin 区、拖拽排序、close-in-place 宽度冻结、tab 间 hairline divider（升级路径：useTabDrag + 冻结/thaw 状态机 + 独立 divider 元素）。
- 主题仅亮/暗两态（cherry 有跟随系统）；状态已持久化在 `userData/host-state.json`。
- `onTabsChanged` 无 unsubscribe、whoami 用一次 `sendSync`（页面级生命周期，demo 取舍；host 重载靠 did-finish-load 重广播补齐）。
- 关闭全部 tab 后是空态，不自动重建首页。
- tsconfig 是单一 program（main/renderer 共用 DOM + node 类型），类型层不区分运行环境 → 升级路径：拆 project references。
- Notes 草稿存在实例的内存 partition 里，随 tab 关闭即清（跨会话持久化应改由主进程存）。
