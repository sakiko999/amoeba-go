/**
 * crawler/resolver —— 按 item.url 统一解析流/弹幕的独立模块。
 *
 * 能力抽离:crawler/rsshub 的输出都只是基础信息(item.url 指向平台详情页)。
 * 本模块对任何来源的条目按 url 路由到平台解析能力,「统一生效」。平台实现已迁入
 * platform-rss-* 插件;此处只有路由表(router)与能力注册表(registry)。
 */
export * from "./router.ts"
export * from "./registry.ts"
export * from "./play.ts"
export * from "./danmaku.ts"
