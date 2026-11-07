/** YouTube 平台播放/弹幕能力：随 channels bundle 加载，由数据服务进程登记进 resolver 注册表。 */

import type { PlatformPlayback } from "@amoeba/resolve"
import { resolveYoutubeStreams, youtubeClient } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "youtube",
  resolveVideoPlay: async (id) => ({ streams: await resolveYoutubeStreams(id) }),
  // YouTube 弹幕是 live chat 轮询(videoId);VOD 无弹幕。
  getDanmaku: (id) => youtubeClient.getDanmaku(id),
}
