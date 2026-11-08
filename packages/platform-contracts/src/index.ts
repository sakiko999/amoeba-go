/**
 * @tabs/platform-contracts —— 平台插件统一契约。
 *
 * 媒体数据模型（MediaMetadata / EpisodeMetadata / SearchResult …）、数据渠道
 * 契约（DataChannel / DataSource / FetchResult …）、provider 接口与错误类型都在
 * 这里；平台插件与域服务（media-library、数据消费方）共用同一份定义，彼此不再
 * 互相依赖。零运行时依赖（只有类型与纯函数）。
 */

export * from "./model";
export * from "./feed";
export * from "./channel";
export * from "./provider";
export * from "./errors";
