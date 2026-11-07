/**
 * @amoeba/core —— 纯后端抓取基座（轻量）。
 *
 * 只保留与订阅管理无关的部分：RSS XML → MediaItem 解析、source provider 注册表
 * （crawler / RSSHub）、错误与日志、媒体数据模型。
 *
 * 订阅管理（订阅 CRUD / 分组 / 刷新编排 / 阅读状态 / 设置 / SQLite 持久化）
 * 已剥离到 @tabs/rss-subscriptions —— 那是面向前端服务的应用层，不属于基础能力。
 */

export { NoChannelError } from "./errors.ts"
export { coreLog, formatError } from "./log.ts"
export { deserializeFeed, deserializeFeedWithTotal } from "./processing/deserialize.ts"
export type { DeserializeContext } from "./processing/deserialize.ts"
export type * from "./types/media-item.ts"
export type * from "./types/live.ts"
