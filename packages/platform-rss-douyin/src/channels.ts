/**
 * 数据服务进程侧插件入口：导出本平台渠道实例数组。
 * rss-service 的 channels.registerPlugin 会把它们登记进服务进程自己的
 * crawler 注册表（不依赖本 bundle 内的注册函数，避免注册进孤立的 Map）。
 */

export { douyinChannels as channels } from "./impl/register.ts";

/** 平台播放/弹幕能力（数据服务进程一并登记，见 resolver/registry）。 */
export { playback } from "./playback.ts"
