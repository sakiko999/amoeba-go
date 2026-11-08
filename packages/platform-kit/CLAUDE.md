# platform-kit

平台扩展的共享工具包。所有 `platform-*` 组件只引这里；媒体/数据契约统一来自
`@tabs/platform-contracts`，本包在入口处整体再导出，平台插件不需要直接依赖 contracts。

## 唯一入口与协议

- `definePlatformExtension({ tags?, ops })` 是平台插件的唯一入口：
  - namespace = `manifest.id` 去掉 `platform-` 前缀（`platform-bili` → `bili`）；
  - 每个 op 的失败统一包成 `PlatformOpError(platform, op, cause)`；
  - `ops` 可以是对象，也可以是 `(ctx) => ops | Promise<ops>`（provider 需要 activate
    期创建时用）。
- 协议是**消费方约定，不是平台定义**：平台可以只实现协议、只注册私有 op，或在协议结果上
  追加私有 op。`api-gateway` 只存 `namespace + ops`，不关心业务形状。
- 现成协议：
  - `feedOps({ channels, playback })` → `channels / fetch / play / danmaku`（feed/直播类）；
  - `metadataOps(provider)` → `search / fetchMetadata / fetchEpisodes`（media-library）。
- 浏览器型渠道（xhs / weibo / douyin）在 `component.json` 的 `inject` 里声明
  `browser.proxy`；`extension.ts` 检测到后动态加载 `runtime/browser/node/browser-cdp.ts`
  并 `installPlatformAppHost({ browser })`。
- v1 卸载语义：网关里的注册不随组件 deactivate 撤销，直到网关自身 reload。

## 包入口与 tsc 边界（重要）

- `package.json` 的 `"."` 指向 `src/runtime/extension.ts`（没有 index barrel）。
- 根 `tsconfig` **有意排除 `src/runtime`**：这套代码由 Bun.build + smoke 校验，不参与根
  `tsc`。因此普通 workspace 包（如 media-manager 独立形态）必须从子路径导入，例如
  `@tabs/platform-kit/provider`；走包根会把 runtime 拉进根 typecheck。
- 其他子路径：
  - `./data`：`apiFetch` 等数据渠道装配工厂；
  - `./provider`：provider HTTP 基座；
  - `./pack`：组件打包辅助（非 workspace 依赖 external + production 安装进产物）；
  - `./runtime/*`：组件 main 内部使用（utils / browser / danmaku / credentials 等）。
- `platform-contracts` 零运行时依赖；改契约时优先改 `packages/platform-contracts/src`，
  再看 kit 的再导出是否仍完整。

## 构建与验证

- 平台组件的 main 由 Bun.build 打成 CJS（宿主 `createRequire` 热重载）；runtime 代码
  不参与根 tsc，改动必须跑宿主 smoke（真实激活 + 网关注册）与平台自己的测试。
- 组件依赖策略见根 CLAUDE.md：非 workspace 依赖可随组件打包安装；workspace 依赖必须内联。
