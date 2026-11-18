/**
 * 统一错误分类（移植自 @amoeba/resolve/errors.ts，保留分类语义与正则顺序）。
 * 宿主侧平台错误已经过网关归类（timeout/platform/…），这里主要兜住引擎
 * （hls.js/dash.js/flv.js）与网络层的裸 Error。
 */

export type ErrorKind =
  | "auth"
  | "risk"
  | "rate-limit"
  | "network"
  | "timeout"
  | "unsupported"
  | "parse"
  | "no-data"
  | "unknown"

/** UI 展示用的短标签（未知错误留给调用方兜底成自己的默认文案）。 */
export const ERROR_KIND_LABEL: Record<ErrorKind, string> = {
  auth: "需要登录",
  risk: "被平台拒绝",
  "rate-limit": "请求过于频繁",
  network: "网络失败",
  timeout: "请求超时",
  unsupported: "不支持的内容",
  parse: "解析失败",
  "no-data": "暂无可用数据",
  unknown: "未知错误",
}

const VALID_KINDS = new Set<string>(Object.keys(ERROR_KIND_LABEL))

/** 错误 → 分类：显式 kind 优先（含跨模块结构化副本），否则按消息保守归类。 */
export function errorKindOf(err: unknown): ErrorKind {
  if (!err) return "unknown"
  const explicit = (err as { kind?: unknown }).kind
  if (typeof explicit === "string" && VALID_KINDS.has(explicit)) {
    return explicit as ErrorKind
  }

  const msg = err instanceof Error ? `${err.name}: ${err.message}` : String(err)
  if (/冷却|too many requests|rate.?limit|429/i.test(msg)) return "rate-limit"
  if (/需要登录|未登录|登录已失效|登录态|cookie|401|unauthorized/i.test(msg)) return "auth"
  if (/风控|验证码|412|418|403|forbidden|禁止访问|risk/i.test(msg)) return "risk"
  if (/timeout|timed out|超时|abort/i.test(msg)) return "timeout"
  if (/ENOTFOUND|ECONNREFUSED|ECONNRESET|EAI_AGAIN|network|网络|fetch failed/i.test(msg)) return "network"
  if (/没有可用数据|没有可用的数据|no data/i.test(msg)) return "no-data"
  if (/无法从 URL|不支持|unsupported|unknown channel|NoChannel/i.test(msg)) return "unsupported"
  if (/解析|parse|JSON|XML/i.test(msg)) return "parse"
  return "unknown"
}
