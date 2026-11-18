/**
 * resolver/danmaku —— 按 item.url 获取弹幕流（注册表分发）。
 *
 * 与 play.ts 对称：路由 → 平台 → 注册表里 platform-* 插件登记的弹幕能力。
 * kind（vod 视频弹幕 / live 直播聊天）从路由推断。
 */

import type { DanmakuStream } from "../danmaku/types.ts"
import { getPlayback } from "./registry.ts"
import { resolveRoute } from "./router.ts"

export type { DanmakuInfo } from "./registry.ts"
import type { DanmakuInfo } from "./registry.ts"

/** 按平台 + id 获取弹幕流（不依赖 url，已确定平台）。 */
export function getDanmakuById(
  platform: "bilibili" | "youtube" | "douyu" | "huya" | "douyin" | "twitch",
  id: string,
  opts?: DanmakuInfo,
): DanmakuStream {
  const playback = getPlayback(platform)
  if (playback?.getDanmaku === undefined) {
    // twitch 一期无弹幕（聊天需宿主 ws text 帧，二期）；未安装插件同此文案。
    throw new Error(`resolver: 平台 ${platform} 无弹幕能力`)
  }
  return playback.getDanmaku(id, opts)
}

/** 按 item.url 获取弹幕流：路由 → id + kind → 平台弹幕函数。 */
export function getDanmakuByUrl(url: string, info?: DanmakuInfo): DanmakuStream {
  const route = resolveRoute(url)
  if (!route) throw new Error(`resolver: 无法从 URL 路由弹幕: ${url}`)
  const kind = info?.kind ?? (route.kind === "video" ? "vod" : "live")
  return getDanmakuById(route.platform, route.id, { ...info, kind })
}
