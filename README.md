# mygo 多 tab 宿主（Go + WebView2）

Go 单二进制桌面宿主：native UI 窗口 + 多 WebView tab，页面组件（静态页面包）+ 内置 Go 服务
（平台数据 / 播放器 / 聚合搜索），宿主进程内无 JS 运行时。旧 Electron 实现整体归档在
`old/`，不参与构建。

## 布局

```
main.go / state.go     窗口装配、组件激活入口、主题存储
components/            内置组件（目录名按类型前缀，manifest 里的 id 仍是组件 id）
  platform-*×12        平台数据服务（含扫码登录配方与密钥元数据）
  service-api-gateway/ 平台 API 注册中心（namespace ops 路由）
  service-browser/     浏览器载具池与 CDP 桥
  service-credentials/ 登录会话、凭据镜像与密钥存取（平台配方在 platform-*）
  service-jsrunner/    签名 JS 隐藏 WebView 执行面（host.js）
  service-player/      播放服务组件（解析平台流，统一交给 player-h5 页面）
  service-search/      聚合搜索服务（多平台 fan-out + 流式 Channel + 结果缓存）
  service-profile/     个人主页服务组件（tab 打开 + 视频/回看列表渠道路由）
  page-credentials/    登录态与密钥页
  page-search/         聚合搜索页（页面组件，vite 产物 out/page）
  page-player-h5/      H5 播放器页（hls.js/dash.js/flv.js，全部播放路径）
  page-profile/        个人主页页（资料卡 + 直播间 + 视频/回看列表）
platform/              运行时共享层（HTTP/缓存/凭证/WS 弹幕/MPD/搜索契约）
registry/              组件注册表（page:// 与 vendor:// 静态服务、manifest 校验、HTML 重写）
services/              组件装配（Kahn 拓扑激活，编译期工厂清单）
tabs/                  tab 生命周期、原生视图（loading/空态/错误页）与菜单
scripts/               构建编排（build.ts：chrome + 页面自动发现 + go，dev/build 子命令）
web/                   宿主侧 TS 资产（bun workspaces）
  chrome/              tab 条前端（React 19 + @demo/ui，产物 embed 进宿主）
  ui/                  @demo/ui 公共组件库（chrome 与页面组件共用）
  sdk/                 @host/sdk 页面类型
  page-vendor/         renderer 共享层（vendor:// 预构建 + import map + CSP hash）
docs/                  当前环境说明与平台协议调研
old/                   Electron 时代全部遗留（归档，不参与构建）
```

## 命令

```sh
bun install
bun run dev        # chrome + 页面组件构建后 go run .
bun run build      # 同上，go build 产物在 tmp/mygo-host/
bun run typecheck  # tsc --noEmit（TS 侧）
bun run lint       # biome
go test ./...      # Go 侧
```

Windows（WebView2）是当前唯一已验收平台；macOS/Linux 尚未做运行时适配。
