# media-manager

媒体库管理器（Electron 前端 + 后端）：扫描 → 海报墙 → 详情 → 刮削 / NFO 编辑 / 目录整理。
从 Tauri/Rust 版本全新改造而来，完整旧实现见本仓库 git 历史（`crates/core`）。WebDAV 未迁移。

## ⚠️ 安全铁律（血换来的，改造后依然有效）

**绝不把破坏性操作指向真实数据。** 本项目前身曾因「junction 当隔离副本 + organize
--apply + rm -rf」链丢失 15GB 视频。

- 整理强制 plan（dry-run）→ 用户确认 → apply；目标已存在一律跳过，绝不覆盖
- 源目录是符号链接/junction 时 `planOrganize` 直接拒绝 —— **不要移除该闸门**
- 写测试只对可丢弃目录（`tmp/`）运行，绝不在真实媒体库上跑 apply
- 刮削/NFO 写回只由用户在当前作品页显式触发
- 凭证只走环境变量（`TMDB_TOKEN`），不经前端传递、不做命令行参数

## 架构

后端（扫描/刮削/NFO/整理/缓存，含安全铁律与领域约定）已迁到主仓库普通子包
`packages/media-library`（`@tabs/media-library`，非 submodule）——能力核心、channel
绑定表与服务组件入口都在那边，本包只剩 UI 与两种形态的装配：

```
src/
├── main/
│   ├── index.ts         独立应用入口：窗口 + app:// + 加固（IPC 在 backend.ts）
│   ├── backend.ts       独立应用组装：@tabs/media-library 的 service + ipcBinding → ipcMain
│   └── pageEntry.ts     页面组件 main（CJS）：ctx.require("media.library") + 绑定渲染侧 IPC
├── preload/             contextBridge → window.media
└── renderer/            前端（React 19 + Tailwind 4 + @demo/ui）
    └── src/             App + TopBar/PosterCard/LibraryGrid/DetailView
                         + ScrapeDialog/NfoEditor/OrganizeDialog
```

- 刮削/整理的安全铁律、领域约定、provider 陷阱见 `packages/media-library/CLAUDE.md`。
- 元数据渠道已插件化：实现分别在 `packages/platform-{tmdb,douban,bangumi}`
  （宿主形态由三个 `kind=service` 组件经 `media.providers` 注册；独立形态由
  `backend.ts` 静态 `providers.register(...)`）。本包只做 UI 与两种形态装配。
- 海报经 IPC 读成 data URL（CSP 不放行 file://，也不放行远程图）。
- 缓存与网络只能在 app ready 后调用（`net.fetch` / `app.getPath`）。

## 命令（在 monorepo 根跑）

```bash
bun run media          # dev（HMR + 主进程自动重启）
bun run media:debug    # 同上，主进程 inspector 9230 + sourcemap（VS Code attach "Media Manager: Main (9230)"）
bun run media:build    # 生产构建 → out/
bun run media:smoke    # 无头冒烟：renderer 加载完 exit 0
bun run --cwd packages/media-library test   # 后端单元测试（bun:test，fixtures 驱动）
bun run --cwd packages/media-manager start --probe-bangumi 8   # 真实网络只读探针（实现见 src/main/probe.ts，provider 来自 platform-bangumi 渠道包）
bun run --cwd packages/media-manager pack:page       # 生成宿主页面组件包 out/page/
bun run --cwd packages/media-library pack            # 生成宿主服务组件包 out/component/
bun run --cwd packages/media-manager dist        # 独立分发（@electron/packager + asar）
```

发布相关：`app.setName("media-manager")` 固定 userData；`app://` 用 readFile 以支持 asar；
打包只含 out/ + 精简 package.json（依赖已全部移到 devDependencies，防止收集未编译 workspace
TS；@tabs/media-library 源码由 Bun.build bundle 进 main.js，运行时零依赖）；
`Media Manager.exe --smoke` 是打包产物的验收命令。

组件包 main 必须是 CJS（宿主用 createRequire 加载并清缓存实现热重载）。后端能力由主仓库
服务组件包 `media-library` 提供（component.json kind=service，provide `media.library`，sqlite
缓存落在服务组件 dataDir = userData/page-data/media-library；另 provide `media.providers`
给三个元数据渠道组件注册 provider）；本页面组件
（kind=page）activate 里 `ctx.require("media.library")` 取服务并绑定渲染侧 IPC，channel
契约不变。独立应用不走服务表：index.ts 仍直接 registerMediaBackend（同一份
createMediaService + bindMediaIpc）。宿主主题经 `window.hostState` 透传（sendSync 初始值 +
host:state 订阅）；嵌入形态由宿主的 pageBridge 注入，独立应用由本包 preload 注入 dark
（用 `app:whoami` 区分）。新增全局状态时从 `@host/sdk` 的 `HOST_STATE_CHANNELS` 取渠道名。

本包是 `electron-tabs-demo` monorepo 的 submodule，公共组件库在 `packages/ui`（@demo/ui）。

renderer 构建走 vendor:// 共享层：pack 时 react/@demo/ui external + import map 注入（manifest 缺失自动回退全内联），dev 不受影响。
