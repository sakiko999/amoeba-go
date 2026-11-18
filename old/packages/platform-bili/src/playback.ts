/** bilibili 平台播放/弹幕能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { biliClient } from "./lib/index.ts"
import { resolveBiliLivePlay } from "./lib/live-play.ts"
import { resolveBiliVideoPlay } from "./lib/video-play.ts"

export const playback: PlatformPlayback = {
  platform: "bilibili",
  routes: [
    // B 站视频(BV 号 / av 号)
    {
      test: /(?:^|[./])bilibili\.com\/video\/(BV\w+)/,
      kind: "video",
      extractId: (u) => u.match(/\/video\/(BV\w+)/)?.[1] ?? null,
    },
    {
      test: /(?:^|[./])bilibili\.com\/video\/(av\d+)/i,
      kind: "video",
      extractId: (u) => u.match(/\/(av\d+)/i)?.[1] ?? null,
    },
    // B 站直播(live.bilibili.com/{roomId})
    {
      test: /(?:^|[./])live\.bilibili\.com\/(\d+)/,
      kind: "live",
      extractId: (u) => u.match(/live\.bilibili\.com\/(\d+)/)?.[1] ?? null,
    },
  ],
  resolveVideoPlay: (id, info) => resolveBiliVideoPlay(id, info),
  resolveLivePlay: (id, info) => resolveBiliLivePlay(id, info),
  getDanmaku: (id, opts) => biliClient.getDanmaku(id, { kind: opts?.kind ?? "vod", cookie: opts?.cookie }),
}
