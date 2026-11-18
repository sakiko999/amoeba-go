/**
 * DashHostLoader — dash.js 自定义 HTTPLoader，分片请求走宿主 fetch 插件（无 CORS，
 * 且 Referer/UA 规则在 hostFetch.ts 合成）。
 *
 * 用法：player.extend("HTTPLoader", () => DashHostLoader({ headers: stream.headers }), true)
 * 在 attachSource 前调用。
 *
 * ⚠️ dash.js 的 extend 用 `Object.create` 实例化 loader——它期望**工厂函数**
 * （function HTTPLoader(cfg){...; return instance}），不是 class。所以这里导出的
 * DashHostLoader 是工厂函数，返回 { load, abort, reset, resetInitialSettings }。
 */

import { type URLLoader } from "dashjs"

import { mediaRequest } from "../hostFetch.ts"
import { log } from "../log/index.ts"

interface DashLoadConfig {
  request?: {
    url?: string
    range?: string
    method?: string
  }
  success?: (data: ArrayBuffer | string, statusText: string, url: string) => void
  error?: (err: unknown) => void
  complete?: (req: unknown, statusText: string) => void
  abort?: (req: unknown) => void
  progress?: (event: { lengthComputable: boolean; loaded: number; total: number }) => void
}

interface DashLoaderInstance {
  load(config: DashLoadConfig): void
  abort(): void
  reset(): void
  resetInitialSettings(): void
}

/**
 * dash.js 自定义 HTTPLoader 工厂（dash.js 用 Object.create 调用，须返回实例对象）。
 * cfg.headers 会并入每个分片请求（与 HlsHostLoader 的 stream.headers 透传对称）。
 */
export function DashHostLoader(cfg?: {
  headers?: Record<string, string>
}): DashLoaderInstance {
  let aborted = false
  let pending: { cancel: () => void } | null = null
  const extraHeaders = cfg?.headers ?? {}

  function load(config: DashLoadConfig): void {
    const req = config.request ?? {}
    const url = req.url ?? ""
    // 每次 load 重置 abort 态——dash.js seek/切档会 abort 旧请求，同实例继续用。
    aborted = false
    if (!url) {
      log.loaderError({ engine: "dash", msg: "no url" })
      config.error?.(new Error("DashHostLoader: no url"))
      return
    }
    const ctrl = new AbortController()
    pending = { cancel: () => ctrl.abort() }
    // blob/data URL（MPD）无 CORS，浏览器原样 fetch；http(s) 分片走宿主。
    if (/^blob:/i.test(url) || /^data:/i.test(url)) {
      fetch(url, { signal: ctrl.signal })
        .then((res) => {
          if (aborted) return
          if (!res.ok) {
            log.loaderHttpError({ engine: "dash", status: res.status, url })
            config.error?.(new Error(`HTTP ${res.status}`))
            return
          }
          // blob 是 MPD（XML）——dash.js 解析器期望字符串，不是 ArrayBuffer。
          return res.text().then((text) => {
            config.success?.(text, res.statusText, url)
            config.complete?.(req, res.statusText)
          })
        })
        .catch((e: unknown) => {
          if (aborted) return
          const msg = e instanceof Error ? e.message : String(e)
          log.loaderError({ engine: "dash", msg })
          config.error?.(e instanceof Error ? e : new Error(msg))
        })
      return
    }
    // http(s) 分片：Range 透传。dash.js 的 request.range 是裸 `start-end`，
    // 统一在 mediaRequest 里补 `bytes=` 前缀。
    const isBinary = !/\.mpd(\?|#|$)/i.test(url)
    mediaRequest(url, {
      binary: isBinary,
      range: req.range,
      signal: ctrl.signal,
      headers: extraHeaders,
    })
      .then((res) => {
        if (aborted) return
        if (res.status < 200 || res.status >= 300) {
          log.loaderHttpError({ engine: "dash", status: res.status, url })
          config.error?.(new Error(`HTTP ${res.status}`))
          return
        }
        // dash.js ManifestLoader/DashParser 期望字符串（会调 indexOf/charCodeAt），
        // 传 ArrayBuffer 会 MANIFEST_LOADER_PARSING_FAILURE。
        config.success?.(res.body, String(res.status), url)
        config.complete?.(req, String(res.status))
      })
      .catch((e: unknown) => {
        if (aborted) return
        const msg = e instanceof Error ? e.message : String(e)
        log.loaderError({ engine: "dash", msg })
        config.error?.(e instanceof Error ? e : new Error(msg))
      })
  }

  function abort(): void {
    aborted = true
    pending?.cancel()
  }

  function reset(): void {
    aborted = true
    pending?.cancel()
  }

  function resetInitialSettings(): void {
    aborted = false
  }

  return { load, abort, reset, resetInitialSettings }
}

/** 供类型标注引用（URLLoader 契约）。 */
export type { URLLoader }
