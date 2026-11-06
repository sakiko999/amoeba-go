<!-- cc-todo:start -->

## TODO / 重要信息（cc-todo 自动维护）

- [!] bun 1.4.4 没有内置 lint 子命令：本项目 lint 用 biome（`biome lint .`），format 用内置 `bun fmt`（默认 80 列）
- [!] 本机网关证书过期（CERT_HAS_EXPIRED）：装需下载二进制的依赖时要临时 `$env:NODE_TLS_REJECT_UNAUTHORIZED='0'`（electron 已这样装好）
- [!] electron 装在 apps/desktop（bun per-workspace 安装），根目录启动必须 `bun run --cwd apps/desktop`，直接 `electron apps/desktop` 找不到命令
- [ ] 用 TMDB_TOKEN 实测 TMDB 渠道，并找网络条件验证豆瓣渠道
- [!] runtime 页面 main 只接受 CJS（宿主 createRequire + require.cache 失效实现热重载）；页面 main 等价完整主进程权限，安装必须警告
- [ ] 手动跑 `bun run dev` 验证 + 菜单：打开媒体库页面、外部页面安装/卸载/重载
- [!] media-manager 发布物是 out/ + package.json（运行时零依赖）；不要为独立分发把 @demo/ui 源码搬出 monorepo，成本高收益为零
- [ ] 待确认后落地 media-manager 独立分发打包（@electron/packager asar → electron-builder）
- [!] `app.setName` 后独立形态的 userData 换到新路径，旧的库根（localStorage）需要重新选择；嵌入形态的 `persist:page-media-manager` 数据不受影响。尚未做安装器（NSIS）、应用图标和 TMDB_TOKEN 设置界面。
- [!] media-manager 独立版 userData 已固定为 %APPDATA%/media-manager（app.setName）；发布后改名会孤儿化 cache 与 Chromium profile
- [ ] 后续：NSIS 安装器/应用图标、TMDB_TOKEN 设置界面、打包版手动启动验收
- [ ] 按评审 P0 落地：渲染侧统一 bridge（registerPreloadScript）、启动时序与失败隔离，再考虑 IPC 鉴权与安装/升级队列
- [!] runtime 页面自带 preload 会替换宿主 preload，渲染侧能力必须由宿主用 registerPreloadScript 叠加，不要要求每个页面重复造桥
- [ ] 若 `library:scan` 耗时仍大：做扫描结果缓存（按库根 + mtime）与首屏先展示上次结果、后台增量刷新
- [!] 修改主进程文件会触发 electron-vite 重启，inspector 会断开并在同一端口重新监听；VS Code 需要重新 attach，`chrome://inspect` 需要重新点一次。生产构建不带 inspector，那时 bug 菜单的「调试主进程」才会带 `--inspect=9229` 重启。

<!-- cc-todo:end -->
