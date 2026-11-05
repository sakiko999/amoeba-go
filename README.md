# Electron 多 tab 架构 demo

bun workspaces monorepo；**每个 tab = 一个子包**，运行时每个 tab 实例 = 主进程管理的独立 `WebContentsView`（独立 webContents / preload / storage / 生命周期）。electron-vite 6 构建（dev 有 HMR + 主进程自动重启），TypeScript 7（原生编译器）typecheck，`bun fmt` + `biome lint`（bun 1.4 没有内置 lint 子命令，用 biome 补位）。

## 架构

```
apps/desktop ──────────────────────────────────────────────┐
│  main (node)                                             │
│   ├─ registry.ts      tab 种类清单（全仓库唯一注册点）      │
│   ├─ tabs.ts          TabManager：WebContentsView 增删/布局│
│   └─ index.ts         窗口 + IPC + --smoke 冒烟           │
│  preload (sandbox)    按角色注入 window.host / window.tab │
│  ui (browser)         tab 条 + “+” 菜单（host 页面）       │
└────────────┬────────────────────────────▲────────────────┘
             │ workspace:*                │ IPC（tabs:changed 状态推送）
             ▼                            │
packages/tab-clock / tab-notes / tab-sysinfo
   manifest（id/title/color）+ index.html + renderer.ts
   经 electron-vite 多页构建到 out/renderer/（产物路径 = 源路径相对仓库根）
```

要点：

- tab 页面与 host 页面互不知道对方，一切状态经主进程（`TabManager.broadcast()` 单向投影给 tab 条）。
- tab 内唯一能力是 preload 暴露的 `window.tab`（setTitle / close / ping / systemInfo）。
- `contextIsolation` + `sandbox` 全开，只加载本地文件；tab 侧 IPC 一律用 sender 定位调用者，不信任页面自报身份。
- 安全默认：拦截一切导航与 `window.open`（主进程 `hardenWebContents`）、权限请求默认全拒、preload 未知角色不注入任何桥。
- 每个 tab 实例独立 webContents + 独立内存 partition（storage 互不可见，关闭即清）；多开同一个 tab kind 互不串数据。
- tab 条为 Chrome 风格 44px（视觉移植自 [cherry-studio](tmp/cherry-studio) 的 `AppShellTabBar`：状态色 color-mix 公式、标题渐隐、hover 展开的 X、关闭 collapse 动画、中键/双击关闭）；cherry-studio 本体是单 renderer 内 React tab（TabsProvider + 每 tab 独立 MemoryRouter + `<Activity>` KeepAlive），与本项目“每 tab 一个 WebContentsView”是两种架构，这里只抄了 UI 层。
- 无系统标题栏：tab 条即标题栏（拖拽区），右侧窗口按钮为原生 `titleBarOverlay`（cherry Windows 同款，高 44 随主题变色）；tab 条尾部有亮/暗切换按钮，主题由主进程唯一持有并 `theme:changed` 广播到 host 与所有 tab（每个 tab 是独立 webContents，各自切换 `<html data-theme>`）。
- “+” 新建与 tab 右键菜单（关闭/关闭其他/关闭右侧）由主进程 `Menu.popup()` 弹出：host 页面 DOM 会被 tab 的原生子视图遮挡，DOM 菜单不可行。
- renderer 加载：dev 走 electron-vite dev server（`ELECTRON_RENDERER_URL`），生产走 `app://` 自定义协议映射 `out/renderer`——ESM 模块脚本需要 http 型 origin，file:// 不行；协议须按 session 注册（每个 tab 的独立 partition 是另一个 session）。
- 调试入口：tab 条的 bug 按钮弹出原生菜单——调试主进程（带 `--inspect=9229` 重启，`chrome://inspect` 连接）、调试启动器界面（host 页面 DevTools）、调试内容页面（当前 tab DevTools）；F12 仍可开关当前焦点 webContents 的 devtools。

## 命令

| 命令                | 作用                                         |
| ------------------- | -------------------------------------------- |
| `bun install`       | 安装（bun workspaces）                       |
| `bun run dev`       | electron-vite dev（HMR、主进程改动自动重启） |
| `bun run build`     | electron-vite build → `apps/desktop/out/`    |
| `bun run smoke`     | 无头冒烟：3 个 tab 全部加载完则退出码 0      |
| `bun run typecheck` | `tsc --noEmit`（TypeScript 7 原生）          |
| `bun run lint`      | `biome lint .`                               |
| `bun run fmt`       | `bun fmt .`                                  |

F12 可开关当前 webContents（host 或任意 tab）的 devtools；tab 条右侧 bug 按钮提供统一的调试菜单（主进程 / 启动器界面 / 内容页）。

## 新增一个 tab

1. 复制 `packages/tab-clock` → `packages/tab-foo`：改 `package.json` 的 name，改 manifest 的 `id`（目录名必须是 `tab-<id>`，与 id 一致）。
2. 在 `apps/desktop` 里 `bun add @tabs/foo`（workspace 链接）。
3. `apps/desktop/src/main/registry.ts` 加一行 import + 数组项。

构建产物、“+” 菜单、IPC、类型全部自动跟进，不需要动其他文件。

## 已知简化（升级路径）

- tab 条未抄 cherry-studio 的 pin 区、拖拽排序、close-in-place 宽度冻结、tab 间 hairline divider（升级路径：useTabDrag + 冻结/thaw 状态机 + 独立 divider 元素）。
- 主题仅亮/暗两态（cherry 有跟随系统），不持久化，重启回深色。
- `onTabsChanged` 无 unsubscribe、whoami 用一次 `sendSync`（页面级生命周期，demo 取舍；host 重载靠 did-finish-load 重广播补齐）。
- 关闭全部 tab 后是空态，不自动重建首页。
- tsconfig 是单一 program（main/renderer 共用 DOM + node 类型），类型层不区分运行环境 → 升级路径：拆 project references。
- Notes 草稿存在实例的内存 partition 里，随 tab 关闭即清（跨会话持久化应改由主进程存）。
