/**
 * 媒体请求统一出口：经宿主 fetch 插件（@mygo-plugins/fetch，请求从 Go 发出）。
 *
 * 一次解决两件事：
 *   1. CORS：googlevideo / mcdn 等分片不带 Access-Control-Allow-Origin，页面直接
 *      XHR 会被拦；Go 侧请求没有这个概念。
 *   2. 请求头：Referer/UA/Origin 都是 forbidden header，页面设不了。原项目由 Rust
 *      在 WebView2 网络层按 mediaReferrer.ts 的规则改写；这里请求本来就从 Go 发出，
 *      规则表与平台返回的 stream.headers 直接合成请求头即可。
 */

import { fetch as hostFetch } from "@mygo-plugins/fetch"

import { videoReferrerFor, xhrOriginFor, xhrUserAgentFor } from "./mediaReferrer.ts"

/** 缺省 UA（上游 HlsHostLoader 同款）：多数 CDN 只要求「像浏览器」。 */
const DEFAULT_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

/** 按 URL 规则合成媒体请求头；extra（stream.headers）优先级最高。 */
export function mediaHeadersFor(
  url: string,
  extra?: Record<string, string>,
): Record<string, string> {
  const headers: Record<string, string> = { "user-agent": DEFAULT_UA }
  const referer = videoReferrerFor(url)
  if (referer) headers.referer = referer
  const origin = xhrOriginFor(url)
  if (origin) headers.origin = origin
  const ua = xhrUserAgentFor(url)
  if (ua) headers["user-agent"] = ua
  return { ...headers, ...extra }
}

export interface MediaResponse {
  status: number
  statusText: string
  url: string
  body: ArrayBuffer | string
}

export interface MediaRequestOptions {
  /** 二进制响应（分片/密钥）还是文本（playlist/MPD）。 */
  binary?: boolean
  /** 已解析的 Range（裸 `start-end` 或 `bytes=…` 都可，统一补前缀）。 */
  range?: string
  signal?: AbortSignal
  headers?: Record<string, string>
}

export async function mediaRequest(
  url: string,
  opts: MediaRequestOptions = {},
): Promise<MediaResponse> {
  const headers = mediaHeadersFor(url, opts.headers)
  if (opts.range) {
    headers.range = opts.range.startsWith("bytes=") ? opts.range : `bytes=${opts.range}`
  }
  const res = await hostFetch(url, { method: "GET", headers, signal: opts.signal })
  const body = opts.binary ? await res.arrayBuffer() : await res.text()
  return { status: res.status, statusText: res.statusText, url: res.url ?? url, body }
}
