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

## 当前宿主组件面（rss 能力暂不启用）

宿主目前只打包 `api-gateway` + `media-library` + `media-manager` + 三个
`platform-media-*`。**gateway 就是数据接口层**：页面/域服务经
`call("<平台>.<操作>")` 取数据，不再依赖 rss 的订阅/抓取/存储/事件栈。
rss 订阅栈源码（rss-service / rss-client / rss-subscriptions）保留在仓库但不由
`scripts/pack-components.ts` 打包（已停用）；`rss-browser` 仍打包 —— 它是
浏览器型数据渠道（xhs/weibo/douyin）的浏览器载具（CDP 代理），与订阅栈无关。

## 平台怎么注册

- **media 平台**（platform-kit `defineMediaPlatformExtension`）：provider 包成
  `<source>` 的 search / fetchMetadata / fetchEpisodes（tags: media.metadata）；media-library 组件
  inject `api.gateway`，刮削时按命名空间**惰性**解析（平台组件晚注册也能被发现，
  不依赖激活顺序）。
- **数据渠道平台**（platform-kit `runtime/extension` 的 `definePlatformExtension`）：
  组件 main 直接把渠道与播放/弹幕能力注册成四个 op —— `channels`（静态描述）、
  `fetch`（channel + info → 结构化 items，cursor 翻页 / passive 被动采集）、
  `play`（url|id → 可播流；URL→id/kind 由平台自带的 playback.routes 路由）、`danmaku`
  （url|id → 弹幕流，subscribe 返回取消函数，进程内消费）。执行体在宿主主进程
  （appHost 由 `runtime/electron-host` 安装；浏览器型渠道 inject `rss.browser`）。

## 命令

```bash
bun run --cwd packages/api-gateway pack   # 产出 out/component/（main.js）
```

宿主 `bun run smoke` 覆盖：api-gateway active + 自检 + 12 个平台命名空间注册
（9 个数据渠道平台各 channels/fetch(/play/danmaku)，3 个 media 平台各
search/fetchMetadata/fetchEpisodes）。
