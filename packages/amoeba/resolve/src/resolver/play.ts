/**
 * resolver/play —— 按 item.url 解析可播流（注册表分发）。
 *
 * 能力抽离：crawler/rsshub 的输出都只是基础信息（item.url 指向平台详情页）。
 * 这里把「URL → 可播流」统一收敛：resolveRoute 路由到平台，实际解析由注册表里
 * platform-rss-* 插件登记的 PlatformPlayback 承担（基座不再 import 平台代码）。
 *
 * info 透传 cookie（登录态档位）：core 层经 sourceInfoFor 注入（info.cookie）；
 * 未显式传的平台由各平台 client 经 credentials provider 回落 settings 登录态。
 */

import type { Stream } from "@amoeba/xml"

import { ResolveError } from "../errors.ts"
import { type PlaybackChapter, getPlayback, type ResolveInfo } from "./registry.ts"
import { type ResolvePlatform, resolveRoute } from "./router.ts"

export type { PlaybackChapter, ResolveInfo } from "./registry.ts"

export interface ResolveResult {
  /** 路由命中的平台 id（视频 bvid/直播 roomId 等；调试/日志用）。 */
  id: string
  streams: Stream[]
  /** 平台给的章节时间点（目前只有 bili 有）；没有就是空/undefined。 */
  chapters?: PlaybackChapter[]
}

/**
 * 按 item.url 解析视频可播流（kind=video 路由）。
 * url 必须是视频详情页；直播按 resolveLivePlayByUrl。
 */
export async function resolvePlayByUrl(url: string, info?: ResolveInfo): Promise<ResolveResult> {
  const route = resolveRoute(url)
  if (!route || route.kind !== "video") {
    throw new ResolveError("unsupported", `resolver: 无法从 URL 解析视频流(仅支持 bilibili 视频/YouTube): ${url}`)
  }
  const { streams, chapters } = await resolveVideoStreams(route.platform as VideoPlatform, route.id, info)
  return { id: route.id, streams, chapters }
}

/** 按 item.url 解析直播可播流（kind=live 路由）。 */
export async function resolveLivePlayByUrl(url: string, info?: ResolveInfo): Promise<ResolveResult> {
  const route = resolveRoute(url)
  // ⚠️ YouTube 直播与点播的 URL 同形(watch?v=<videoId> / youtu.be/<id>),路由表只能
  // 判为 kind="video";而 `youtube:live` channel 产出的是 kind="live" 条目,播放走本函数
  // → 若严格要 route.kind==="live" 则 youtube:live **100% 无法播放**(抛「不是直播链接」)。
  // 故放行 youtube 的 video-route,复用视频解析(YouTube 直播流由平台实现内部判为 HLS)。
  if (route && route.kind === "video" && route.platform === "youtube") {
    const { streams } = await resolveVideoStreams("youtube", route.id, info)
    return { id: route.id, streams }
  }
  if (!route || route.kind !== "live") {
    throw new ResolveError("unsupported", `resolver: 无法从 URL 解析直播流(需直播路由): ${url}`)
  }
  const streams = await resolveLiveStreams(route.platform as LivePlatform, route.id, info)
  return { id: route.id, streams }
}

/** 视频平台(video 路由可命中的平台)。 */
type VideoPlatform = Extract<ResolvePlatform, "bilibili" | "youtube">

/** 按视频平台 + id 解析可播流（bili 顺带章节；YouTube 无章节数据）。 */
export async function resolveVideoStreams(
  platform: VideoPlatform,
  id: string,
  info?: ResolveInfo,
): Promise<{ streams: Stream[]; chapters?: PlaybackChapter[] }> {
  const playback = getPlayback(platform)
  if (playback?.resolveVideoPlay === undefined) {
    throw new ResolveError(
      "unsupported",
      `resolver: 平台 ${platform} 未注册视频解析（需安装对应 platform-rss 插件）`,
    )
  }
  return playback.resolveVideoPlay(id, info)
}

/** 直播平台(live 路由可命中的平台)。 */
type LivePlatform = Extract<ResolvePlatform, "bilibili" | "douyu" | "huya" | "douyin" | "twitch">

/** 按直播平台 + roomId 解析可播流。 */
export async function resolveLiveStreams(platform: LivePlatform, id: string, info?: ResolveInfo): Promise<Stream[]> {
  const playback = getPlayback(platform)
  if (playback?.resolveLivePlay === undefined) {
    throw new ResolveError(
      "unsupported",
      `resolver: 平台 ${platform} 未注册直播解析（需安装对应 platform-rss 插件）`,
    )
  }
  return playback.resolveLivePlay(id, info)
}
