# media-manager

媒体库管理器：扫描本地媒体库（Kodi/Jellyfin 式目录结构）→ 海报墙 → 季集详情 →
刮削 / NFO 编辑 / 目录整理。Electron 前端 + 后端结构（React 19 + Tailwind 4 + shadcn）。

```
src/main       后端：扫描、NFO 读写、刮削（Bangumi/TMDB/豆瓣）、整理、SQLite 缓存
src/preload    contextBridge → window.media
src/renderer   前端：海报墙 / 详情 / 刮削面板 / NFO 编辑 / 整理确认
src/shared     三方契约类型 + provider 中间模型
```

## 功能

- **浏览**：库根一级子目录 = 作品；`Season NN` 季目录优先于扁平布局；海报 poster/folder 约定
- **刮削**：Bangumi（免 token，默认）/ TMDB（`TMDB_TOKEN`）/ 豆瓣（按 ID、无需 token）
  - 作品 NFO + 每集 NFO + poster.jpg；Kodi 规范（uniqueid、嵌套 ratings、aired/premiered 区分）
- **NFO 编辑**：读取作品 NFO → 表单 → 写回（演员/外部 ID 原样保留）
- **整理**：plan（dry-run）→ 用户确认 → apply；规范命名、特典归 Season 00、目标冲突跳过、
  符号链接目录拒绝；`EXDEV` 跨盘复制成功后才删源
- **缓存**：node:sqlite，元数据响应缓存 7 天
- **未迁移**：WebDAV（旧 Rust 版本有，Electron 版按需求不含）

## ⚠️ 写操作安全

刮削 / 编辑 / 整理会改动磁盘。整理永远先展示计划再执行，目标已存在一律跳过；测试只对
可丢弃目录运行（`tmp/`）。绝不对真实媒体库跑移动测试。

## 命令（在 monorepo 根）

```bash
bun run media              # dev（HMR + 主进程自动重启）
bun run media:build        # 生产构建 → out/
bun run media:smoke        # 无头冒烟：renderer 加载完 exit 0
bun run --cwd packages/media-manager test                    # 单元测试
bun run --cwd packages/media-manager dist                    # 独立分发（asar）
bun run --cwd packages/media-manager start --probe-bangumi 8 # 真实 Bangumi 链路探针
```

独立分发产物：`release/Media Manager-win32-x64/Media Manager.exe`。只打包 `out/` 与精简
`package.json`，不含 `node_modules`（renderer 已 bundle，main/preload 只依赖 electron 与
Node 内建）。验收：`Media Manager.exe --smoke` 必须打印 `SMOKE OK` 并退出 0。
`app://` 读取走 Node fs（支持 asar）；`app.setName("media-manager")` 固定 userData 为
`%APPDATA%/media-manager`；打包版带单实例锁并移除默认应用菜单。暂无安装器/图标
（后续可接 electron-builder）。

探针走主进程 `net.fetch`（系统证书），只读；`--probe-bangumi-apply <可丢弃目录> <id>`
会写 NFO/海报，务必用临时目录。

## 作为宿主 runtime 页面（apps/desktop）

`bun run --cwd packages/media-manager pack:page` 产出 `out/page/`（renderer + preload +
CJS main 扩展 + page.json），根目录执行 `bun run pages:prepare` 会构建并同步到
`apps/desktop/out/pages/media-manager`。宿主按 `page://media-manager/…` 加载页面，main 扩展
（`src/main/pageEntry.ts`）注册的媒体后端 IPC 只接受该页面来源（senderFrame 鉴权）。
独立应用（`bun run media`）与页面模式共用 `src/main/backend.ts`。
宿主主题通过 `window.hostState`（`initial` + `onChanged`）透传，renderer 在首帧前切换
`.dark` 类。嵌入形态由宿主的 `pageBridge` 注入该全局；独立应用由本包 preload 注入 dark。

本包是 `electron-tabs-demo` monorepo 的 submodule，公共组件库在 `packages/ui`（@demo/ui）。
