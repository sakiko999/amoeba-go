/**
 * @tabs/platform-contracts —— 平台插件统一契约。
 *
 * 媒体数据模型（MediaMetadata / EpisodeMetadata / SearchResult …）、provider
 * 接口与错误类型都在这里；platform-media-* 插件与 media-library 域服务共用
 * 同一份定义，彼此不再互相依赖。零运行时依赖（只有类型与纯函数）。
 */

export * from "./model";
export * from "./feed";
export * from "./provider";
export * from "./errors";
