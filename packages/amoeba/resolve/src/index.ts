/**
 * @amoeba/resolve —— 纯后端抓取基座的平台无关层。
 *
 * 定位（platform 插件化之后）：本包只保留与具体平台无关的能力——HTTP 基座、
 * 浏览器调度（CDP）、通用工具、弹幕流通用件、播放/弹幕的路由与注册表、
 * cookie 凭证收口。平台实现（client/签名/登录/播放解析/弹幕协议）全部在
 * platform-* 插件包的 src/lib，经注册表在数据服务进程登记。
 *
 * 对外契约：按 item.url 的懒解析（resolvePlayByUrl/getDanmakuByUrl，注册表分发）
 * + 平台无关的抓取工具（插件 lib 与 crawler channel 共用）。
 *
 * 类型边界：SourceInfo/LoginResult 在此镜像声明（结构兼容 crawler，不依赖它）。
 */
/** 平台解析 / 抓取参数与 crawler 结构一致(channel info / resolver info)。 */
export type SourceInfo = Record<string, string>

/** 扫码登录结果(与 crawler LoginResult 结构一致)。 */
export interface LoginResult {
  cookie: string
  user_id?: string
  alreadyLoggedIn?: boolean
  /** 续期凭证(bili 扫码返回;bili 可自动保活,见 docs/platform-login-research.md)。 */
  refreshToken?: string
}

export type { DanmakuItem, DanmakuOptions, DanmakuStream } from "./danmaku/index.ts"

export * from "./danmaku/index.ts"
export * from "./errors.ts"
export * from "./resolver/index.ts"

// ── 平台无关的凭证收口与平台客户端契约（平台实现在 platform-* 插件） ──
export * from "./platform/credentials.ts"
export type * from "./platform/types.ts"

// ── 公共工具(crawler channel 也用:httpText/now/log/utils) ──
export * from "./host.ts"
export * from "./log.ts"
export * from "./media-referrer.ts"
export * from "./utils/index.ts"

// ── 浏览器模拟(weibo/xhs channel 抓取用,CDP 附加真实 Edge) ──
export * from "./browser/cdp.ts"
export * from "./browser/scheduler.ts"
