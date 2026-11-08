/**
 * Twitch 直播流列表 source 装配(热门 / 分区共用)。
 *
 * `fetch` = 首页;`fetchMore` = 游标翻页。⚠️ core 首次翻页**不传首页游标**
 * (首页游标从未经过 core),所以首次 fetchMore 要先取一次首页拿 `endCursor`,
 * 再取下一页 —— 两次请求换正确分页(Twitch 限流 120 RPM,足够)。
 *
 * 只提供 fetch / fetchMore 两项能力;不实现 resolveDetail / fetchPassive / Loginable。
 */

import { formatError } from "@tabs/platform-kit/runtime/log"
import { now } from "@tabs/platform-kit/runtime/host";
import { log } from "@tabs/platform-kit/runtime/log";
import { TwitchStreamPage } from "../lib/index.ts";
import { type Live, type SerializeOptions } from "@tabs/platform-kit/data"

import type { Pageable, DataSource } from "@tabs/platform-kit/data"
import { twitchStreamToLive } from "./live.ts"

export interface TwitchStreamsSourceOptions {
  /** 该 source 的 channel key(item.sourceId)。 */
  sourceId: string
  /** @deprecated XML 时代的标题/链接选项（死配置，保留兼容旧调用点）。 */
  channelOptions: () => SerializeOptions
  /** 取一页(after 省略 = 首页)。 */
  fetchPage: (after?: string) => Promise<TwitchStreamPage>
}

export function createTwitchStreamsSource(opts: TwitchStreamsSourceOptions): DataSource & Partial<Pageable> {
  const toItems = (page: TwitchStreamPage): Live[] => {
    const fetchedAt = now()
    return page.streams
      .map((s) => twitchStreamToLive(s, opts.sourceId, fetchedAt))
      .filter((i): i is Live => i !== null)
  }

  // 首页/翻页失败都留痕(与 factory 的 apiFetch 对等),再 rethrow 给 core 隔离单源失败。
  const run = async <T>(fn: () => Promise<T>, o: SerializeOptions): Promise<T> => {
    try {
      return await fn()
    } catch (e) {
      log.crawler.fetchError({ source: o.channelTitle ?? "", message: formatError(e) })
      throw e
    }
  }

  return {
    fetch: async () => {
      const o = opts.channelOptions()
      return run(async () => ({ items: toItems(await opts.fetchPage(undefined)) }), o)
    },
    fetchMore: async (cursor?: string) => {
      const o = opts.channelOptions()
      return run(async () => {
        let page: TwitchStreamPage
        if (cursor) {
          page = await opts.fetchPage(cursor)
        } else {
          // 首次翻页:先取首页的 endCursor,再取下一页;首页已到底则返回空结果(无 cursor)。
          const first = await opts.fetchPage(undefined)
          if (!first.hasMore || !first.endCursor) return { items: [] }
          page = await opts.fetchPage(first.endCursor)
        }
        return {
          items: toItems(page),
          ...(page.hasMore && page.endCursor ? { cursor: page.endCursor } : {}),
        }
      }, o)
    },
  }
}
