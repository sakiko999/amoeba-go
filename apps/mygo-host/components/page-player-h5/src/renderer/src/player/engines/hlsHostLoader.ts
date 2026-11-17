/**
 * HlsHostLoader — hls.js 自定义 loader，所有请求走宿主 fetch 插件（hostFetch.ts）。
 *
 * 为什么全部走宿主（上游只对 googlevideo 这么做）：各 CDN 的 CORS 与防盗链
 * 表态不一致，页面侧无法预判；从 Go 发请求统一无 CORS、且能带 Referer/UA
 * （原项目由 Rust 网络层改写，规则表见 ../mediaReferrer.ts）。
 *
 * 用法：hls.config.loader = HlsHostLoader
 */

import Hls, {
  type Loader,
  type LoaderCallbacks,
  type LoaderConfiguration,
  type LoaderContext,
  type LoaderStats,
  LoadStats,
} from "hls.js"

import { mediaRequest } from "../hostFetch.ts"
import { log } from "../log/index.ts"

/** hls.js 的 Loader 类型作为类实现（`new (config: HlsConfig) => Loader<LoaderContext>`）。 */
export class HlsHostLoader implements Loader<LoaderContext> {
  context: LoaderContext | null = null
  stats: LoaderStats = new LoadStats()
  private aborted = false

  constructor(_config: Hls["config"] extends object ? Hls["config"] : never) {}

  load(
    context: LoaderContext,
    _config: LoaderConfiguration,
    callbacks: LoaderCallbacks<LoaderContext>,
  ): void {
    this.context = context
    // ⚠️ 不要 `this.stats = new LoadStats()`——hls.js 在 `new Loader()` 后、`load()`
    // 前就捕获了 `frag.stats = loader.stats` 引用。重新赋值会让带宽采样写进死对象。
    const stats = this.stats
    stats.loading.start = 0
    stats.loading.first = 0
    stats.loaded = 0
    stats.total = 0
    // ABR 采样依赖这三个时间戳；即便当前锁最高档，也保持与上游同样的打点。
    const t0 = performance.now()
    stats.loading.start = t0
    stats.loading.first = t0
    const { url, responseType, headers, rangeStart, rangeEnd } = context
    const isBinary = responseType === "arraybuffer"
    const range =
      rangeStart !== undefined && rangeEnd !== undefined
        ? `bytes=${rangeStart}-${rangeEnd}`
        : undefined

    mediaRequest(url, { binary: isBinary, range, headers })
      .then((res) => {
        if (this.aborted) return
        if (res.status < 200 || res.status >= 300) {
          log.loaderHttpError({ engine: "hls", status: res.status, url })
          callbacks.onError(
            { code: res.status, text: `HTTP ${res.status}` },
            context,
            null,
            this.stats,
          )
          return
        }
        this.stats.loading.end = performance.now()
        this.stats.loaded = isBinary
          ? (res.body as ArrayBuffer).byteLength
          : (res.body as string).length
        this.stats.total = this.stats.loaded
        callbacks.onSuccess({ url, data: res.body }, this.stats, context, null)
      })
      .catch((err: unknown) => {
        if (this.aborted) return
        const msg = err instanceof Error ? err.message : String(err)
        log.loaderError({ engine: "hls", msg })
        callbacks.onError({ code: 0, text: msg }, context, null, this.stats)
      })
  }

  abort(): void {
    this.aborted = true
  }

  destroy(): void {
    this.aborted = true
  }
}
