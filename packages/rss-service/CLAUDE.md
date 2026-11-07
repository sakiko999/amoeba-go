# rss-service

RSS 数据服务组件（`@tabs/rss-service`，主仓库普通子包）：承载 amoeba 数据服务，
为宿主提供 `kind=service` 组件（`rss.endpoint`）。

## 源码已移植（不再依赖外部服务/外部仓库产物）

数据服务本体来自 tauri-rss（amoeba），**源码已整体移植进本仓库**：

- `runtime/`：服务入口与宿主注入（`main.ts` / `server.ts` / `host.ts` /
  `db-lock.ts` / `sqlite-storage.ts` / `log-shim.ts`，源自 tauri-rss 的
  `apps/cli/src/{service,store,host}`）
- `packages/amoeba/{core,crawler,resolve,host,xml,platform,log,service-protocol}`：
  数据服务依赖栈（**保留原 `@amoeba/*` 包名**，源码原样移植；bun workspace 成员）
- **构建**：`scripts/pack.ts` 在本仓库把 `runtime/main.ts` 打成 `out/component/service.js`
  （bun target 单文件，与 tauri-rss 的 service:build 同构），把 `src/entry.ts` 打成
  组件入口 `main.js`。**没有** `X:\tauri-rss` 路径兜底。
- **升级**：重新把 tauri-rss 对应目录复制覆盖（packages/amoeba/* + runtime/*），
  再 `bun run --cwd packages/rss-service pack`。vendor 目录不参与本项目
  typecheck / biome（见 tsconfig include 与 biome.json 排除项）。

## 运行时形态（临时方案：仍 spawn bun）

amoeba 数据服务是**独立 Bun 进程**：单写者独占 SQLite（`db-lock` 按库文件判锁）、
HTTP/WS RPC（`@amoeba/service-protocol`）+ owner/client token 鉴权 + rev 事件。
本组件做生命周期宿主：

- `activate`：定位 bun（`AMOEBA_BUN` → PATH）与组件包内 `service.js`（`AMOEBA_SERVICE_JS`
  可覆盖，调试用）→ spawn（`--port 0`，dataDir = `userData/page-data/rss-service/data`，
  `--parent-pid` 宿主崩溃时服务自杀）→ 解析 stdout `READY <port>` →
  `ctx.provide("rss.endpoint", {endpoint, health, stop})`。
- `deactivate`：`POST /v1/shutdown`（owner token）→ 等退出 → kill 兜底（幂等）。

**bun 运行时是当前临时依赖**；迁移到进程内（`bun:sqlite` → `node:sqlite`、去掉
HTTP/WS 服务层、事件直连）是后续工作，届时 `runtime/` 与 `packages/amoeba/host/node`
的存储适配层是主要改动面。

## 组件拆分（rss 域三个 kind=service 子包，按职责分）

| 组件包 | provides | inject | 职责 |
| --- | --- | --- | --- |
| `rss-browser` | `rss.browser`（port/targetCount） | — | Electron 隐藏浏览器 + CDP 代理（替代 WebView2 sidecar） |
| `rss-service`（本包） | `rss.endpoint`（baseUrl/port/token） | `rss.browser` | 数据服务进程生命周期（spawn/READY/shutdown） |
| `rss-client` | `rss.client`（call/health/endpoint） | `rss.endpoint` | 方法化 RPC 访问层（token 不出组件边界） |

宿主按 provides/inject 拓扑激活（browser → service → client）；任一服务 reload 时
依赖它的组件会被级联停用并在恢复后重激活。

## 验证

```bash
bun run --cwd packages/rss-service pack   # 本仓库构建 out/component/（main.js + service.js）
bun run --cwd packages/rss-service test   # 真实进程生命周期集成（spawn 本地 service.js）
```

宿主 `bun run smoke` 会真实拉起 browser 代理（CDP 自检）与数据服务（三服务 active）。
真实渠道抓取：dev 宿主里经 rss-client `call("probe.fetch", {channelKey:"weibo:user:search", info:{keyword:"微博"}})`，
已验证返回真实 RSS（~7KB / ~3s）。强反爬平台（xhs/douyin）需真机登录验证。
