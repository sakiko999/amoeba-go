/**
 * @tabs/platform-kit/runtime —— 平台插件共用的运行时基座（项目自有）。
 *
 * 内容源自 amoeba 的 resolve/host 通用层，按平台插件需要收敛：
 * HTTP/时钟、日志、通用工具、弹幕原语、平台客户端/凭证契约、CDP 浏览器调度、
 * 被动采集缓冲、播放/弹幕路由与注册表。
 *
 * 平台插件只依赖本模块与 @tabs/platform-contracts，不再直接依赖 amoeba。
 */

export type { SourceInfo, LoginResult } from "../data.ts";

export * from "./danmaku/index.ts";
export * from "./errors.ts";
export * from "./playback.ts";

export * from "./platform/credentials.ts";
export type * from "./platform/types.ts";

export * from "./host.ts";
export * from "./log.ts";
export * from "./media-referrer.ts";
export * from "./utils/index.ts";

export * from "./browser/cdp.ts";
export * from "./browser/scheduler.ts";
