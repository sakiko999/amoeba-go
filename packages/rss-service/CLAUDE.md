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
  `--no-browser --no-cookie-sync`，`--parent-pid` 让服务随宿主退出自杀）→ 解析 stdout
  `READY <port>` → `ctx.provide("rss.endpoint", {endpoint, health, stop})`。
- `deactivate`：`POST /v1/shutdown`（owner token）→ 等退出 → kill 兜底（幂等）。

**v1 限制**：`--no-browser` = 浏览器渠道（xhs/douyin 登录类反爬）不可用，RSS 与 API
渠道不受影响；接入 Electron 侧抓取浏览器是后续工作。升级 tauri-rss 的服务 =
重新 `service:build` 后重跑本包 `pack`。

消费方（未来的 RSS 页面组件）：manifest `inject: ["rss.endpoint"]`，activate 里
`ctx.require` 拿 `{baseUrl, port, token}`，renderer 用 `@amoeba/service-client` 的
RemoteDataLayer 直连 —— RPC/事件/鉴权复用服务协议，**owner token 不注入 renderer**
（泄漏即全权 RPC，含清库）。

## 命令（monorepo 根）

```bash
bun run --cwd packages/rss-service test   # 真实进程生命周期集成测试（需 bun + service.js，缺失自动 skip）
bun run --cwd packages/rss-service pack   # 产出 out/component/（含 service.js，自包含）
```

宿主 smoke 会真实拉起本服务（builtin 组件 activate 必须成功），因此构建机需要 bun
在 PATH 且能定位 service.js。
