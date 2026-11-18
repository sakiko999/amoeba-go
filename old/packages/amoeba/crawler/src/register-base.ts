/**
 * 基础渠道注册（基础能力插件的一部分）。
 *
 * 只注册与平台无关的渠道：原生 RSS 直链（内置清单/任意 URL）与播客。
 * 平台渠道（bili/weibo/xhs/douyin/douyu/huya/twitch/youtube/apple 目录）
 * 已拆为扩展插件：各平台 `channels/<platform>/register.ts` 由 rss-channel-*
 * 组件在运行时登记（electron-tabs-demo 的 rss-service `channels.registerPlugin`），
 * 本文件刻意不 import 它们 —— 基础 bundle 不含平台代码。
 */

import { RawRssChannel, RSS_BUILTIN_FEEDS, RssPodcastChannel } from "./channels/rss/index.ts"
import { registerChannel } from "./index.ts"

/** 注册基础渠道（无幂等守卫，由 index.ts 的惰性注册调用）。 */
export function registerBaseChannels(): void {
  // ── 原生 RSS 直链(批量注册内置清单,kind 标注在清单里)──
  for (const f of RSS_BUILTIN_FEEDS) {
    registerChannel(new RawRssChannel(`rss:${f.id}`, f.title, f.kind, f.url))
  }
  // ── 播客 RSS(解析 enclosure/itunes → AudioItem)──
  registerChannel(RssPodcastChannel)
  // 通用 RSS 直链(任意 URL,透传上游 XML)——OPML 导入与手工订阅用它。
  registerChannel(new RawRssChannel("rss:url", "RSS 直链", "article"))
}
