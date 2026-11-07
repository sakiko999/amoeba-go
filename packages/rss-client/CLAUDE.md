# rss-client

RSS 数据客户端组件（`@tabs/rss-client`）：`inject rss.endpoint` → `provide rss.client`。

- `call(method, params)`：POST `/v1/rpc`（方法名 = `@amoeba/service-protocol` 的
  RpcMethodMap，如 `subscriptions.list` / `refreshAll` / `reading.markRead`），
  Auth 头由组件注入；失败抛 `[kind] message`。
- `endpoint()`：`{ baseUrl, port }` —— **刻意不含 owner token**；需要 WS 事件流的
  页面 renderer 应经页面 main 自建 `@amoeba/service-client` 的 RemoteDataLayer
  （client token 由服务 `/v1/hello` 下发，见 tauri-rss 协议文档）。
- `health()`：`/v1/health`。

owner token 只存在于本组件与 rss-service 之间，不向 renderer 暴露（泄漏即全权 RPC，
含清库/重置）。服务 reload 时宿主级联停用本组件并在恢复后重新激活，`call` 不缓存连接。

v1 只做薄 RPC 封装；完整 RemoteDataLayer（镜像/rev 事件/断线重连）留给未来页面组件
按 `@amoeba/service-client` 接入，不在本组件重复实现。
