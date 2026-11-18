# rss-subscriptions

RSS 订阅管理（`@tabs/rss-subscriptions`）：**面向前端服务的应用层**，从 `@amoeba/core`
剥离而来。纯后端抓取基座（渠道契约/注册表/XML 解析/source 注册表）留在 `@amoeba/core`；
本包只做订阅语义与持久化：

- `data-layer.ts`：订阅 CRUD / 分组 / 刷新编排（TTL 缓存、被动优先、防御等级）/
  阅读状态 / 设置 / 播放解析 API 装配
- `repo/*`：subscription / settings / reading / content-cache / body-cache 持久化
- `store/media-store.ts`：内存查询镜像（服务端快照与事件投影复用）
- `db/schema.ts`：SQLite DDL（数据服务与 desktop 共用）
- `types/*`：订阅 / 设置 / 阅读 / 查询 / 刷新结果 / 渠道信息 / 播放模型
- `pure.ts`：零依赖入口（desktop 只走这里，不拉 crawler/resolve）

依赖方向：`rss-service` → `@tabs/rss-subscriptions` → `@amoeba/core` + `@amoeba/crawler`
+ `@amoeba/resolve` + `@amoeba/platform` + `@amoeba/xml`。反转即环。

## 为什么不在根 tsc

本包 import 的 `@amoeba/*` 是 vendor 代码（无分号风格、`allowImportingTsExtensions`），
根 `tsconfig` 排除了它（与 platform-rss 实现同理）。类型与打包由 Bun.build（rss-service
的 service.js）与 `bun run --cwd packages/rss-service test` 覆盖。如果以后把 vendor 全部
升成项目自有代码，再把它加回 include。

## 命令

```bash
bun run --cwd packages/rss-service pack   # 构建 service.js（含本包）
bun run --cwd packages/rss-service test   # 真实进程生命周期集成
```

`src/example/*.check.ts` 是移植自 core 的自检脚本（真实网络/宿主注入），不参与构建与
biome（保持 vendor 原样），需要时用 `bun run <file>` 手动执行。
