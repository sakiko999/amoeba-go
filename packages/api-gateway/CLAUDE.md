# api-gateway

统一平台 API 注册中心（`@tabs/api-gateway`，kind=service 组件，provide
`api.gateway`）。所有 platform 插件把
自己的能力以「命名空间.操作」注册到这里；消费方经 `call` / `list` 使用。

- **注册模型**：`register({ namespace, label, tags?, ops })`；namespace = 平台 id
  （单段小写，如 `tmdb` / `bili`，平台就是平台）；ops 是 `名字 → { description?, invoke }`。
  同 namespace 重复注册整体覆盖。`call("tmdb.search", params)` 分发，
  `list()` 返回结构化清单（含 tags，内省/过滤用）。
- **自环自检**：activate 时注册 `gateway.ping` 并调用，宿主 smoke 日志可见
  （`self-check ok`）。
- **边界**：本组件只有注册表与分发。rss 域的订阅/抓取/存储/事件全部在
  rss-service 的数据服务进程里；media 域的刮削/NFO/整理在 media-library。
  平台插件是「注册方」，不是「被包含方」。

## 为什么（暂时）不是 GraphQL（决策记录）

实质理由：
1. 查询形态过程式（搜索 / 按 id 取详情 / 分页），无图遍历与字段裁剪需求；
2. 消费端是宿主 IPC 而非 HTTP，GraphQL 生态（持久化查询/缓存/devtools）用不上。

加分项：组件可以自带 npm 依赖（platform-kit pack 会 external 并装进产物
node_modules），引 graphql-js 已无分发障碍，只是没必要——仍由前两条决定。

演进路径：注册表 handler `(params) => Promise<result>` 与 GraphQL resolver
同构，未来需要字段裁剪时加「注册表 → SDL」层即可，插件不用改。

## 平台怎么注册

- **media 平台**（platform-kit `defineMediaPlatformExtension`）：provider 包成
  `<source>` 的 search / fetchMetadata / fetchEpisodes（tags: media.metadata）；media-library 组件
  inject `api.gateway`，刮削时按命名空间**惰性**解析（平台组件晚注册也能被发现，
  不依赖激活顺序）。
- **rss 平台**（`defineRssPlatformExtension`）：执行体（渠道 + playback）仍经
  `rss.extend` 注册进数据服务进程；同时 pack 阶段从 channels bundle 提取渠道
  静态描述生成 `api.json`，组件 main 据此注册 `<平台>.channels` 元数据 API。
  （抓取执行型 API 未来可由网关代理到 rss 数据服务，v1 未做。）

## 命令

```bash
bun run --cwd packages/api-gateway pack   # 产出 out/component/（main.js）
```

宿主 `bun run smoke` 覆盖：api-gateway active + 自检 + 12 个平台命名空间注册。
