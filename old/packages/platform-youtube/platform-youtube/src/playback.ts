/** YouTube 平台播放/弹幕能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { resolveYoutubeStreams, youtubeClient } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "youtube",
  // 直播与点播 URL 同形(watch?v=/youtu.be)，统一按 video 路由；
  // 直播流由 resolveVideoPlay 内部判为 HLS（旧 resolveLivePlayByUrl 同语义）。
  routes: [
    {
      test: /(?:^|[./])youtube\.com\/watch\?v=([^&]+)/,
      kind: "video",
      extractId: (u) => {
        try {
          return new URL(u).searchParams.get("v")
        } catch {
          return null
        }
      },
    },
    {
      test: /(?:^|[./])youtu\.be\/([^?/]+)/,
      kind: "video",
      extractId: (u) => u.match(/youtu\.be\/([^?/]+)/)?.[1] ?? null,
    },
  ],
  resolveVideoPlay: async (id) => ({ streams: await resolveYoutubeStreams(id) }),
  // YouTube 弹幕是 live chat 轮询(videoId);VOD 无弹幕。
  getDanmaku: (id) => youtubeClient.getDanmaku(id),
}
