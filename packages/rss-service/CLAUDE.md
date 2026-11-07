# rss-service

RSS 数据服务组件（`@tabs/rss-service`，主仓库普通子包）：把 tauri-rss（amoeba）的
**数据服务**承载为 electron-tabs-demo 宿主的 `kind=service` 组件。

## 形态与边界（有意为之，别"优化"成进程内）

amoeba 数据服务是**独立 Bun 进程**：单写者独占 SQLite（`store/db-lock` 按库文件判锁）、
自带 HTTP/WS RPC（`@amoeba/service-protocol`）+ owner/client token 鉴权 + rev 事件。
桌面端本来就是它的客户端。本组件**不重写、不分叉**这套实现，只做生命周期宿主：

- `activate`：定位 bun（`AMOEBA_BUN` → PATH）与 service.js（`AMOEBA_SERVICE_JS` →
  组件包内 `service.js`（pack 时从 tauri-rss 复制）→ `X:\tauri-rss\dist\service\service.js`
  开发兜底）→ spawn（`--port 0`，独立 dataDir = `userData/page-data/rss-service/data`，
  `--parent-pid` 让服务随宿主退出自杀）→ 解析 stdout `READY <port>` →
  `ctx.provide("rss.endpoint", {endpoint, health, stop})`。
- `deactivate`：`POST /v1/shutdown`（owner token）→ 等退出 → kill 兜底（幂等）。

## 组件拆分（rss 域三个 kind=service 子包，按职责分）

| 组件包 | provides | inject | 职责 |
| --- | --- | --- | --- |
| `rss-browser` | `rss.browser`（port/targetCount） | — | Electron 隐藏浏览器 + CDP 代理（替代 WebView2 sidecar） |
| `rss-service`（本包） | `rss.endpoint`（baseUrl/port/token） | `rss.browser` | 数据服务进程生命周期（spawn/READY/shutdown） |
| `rss-client` | `rss.client`（call/health/endpoint） | `rss.endpoint` | 方法化 RPC 访问层（token 不出组件边界） |

宿主按 provides/inject 拓扑激活（browser → service → client）；任一服务 reload 时
依赖它的组件会被级联停用并在恢复后重激活（宿主 registry 的 cascade 语义）。

**v1 限制**：`--no-cookie-sync`；浏览器渠道的登录态存 Electron session 分区
（`persist:rss-crawler`，见 rss-browser/CLAUDE.md）。升级 tauri-rss 的服务 =
重新 `service:build` 后重跑 `bun run pages:prepare`。

## 验证

```bash
bun run --cwd packages/rss-service test   # 真实进程生命周期集成（需 bun + service.js，缺失自动 skip）
bun run --cwd packages/rss-service pack   # 产出 out/component/（含 service.js，自包含）
```

宿主 `bun run smoke` 会真实拉起 browser 代理（含 CDP 自检）与数据服务（three services
必须 active）。真实渠道抓取（如 `weibo:user`）需在 dev 宿主里经 rss-client 的
`call("probe.fetch", {channelKey, info})` 验证 —— 强反爬平台（xhs/douyin）还需真机登录验证。
