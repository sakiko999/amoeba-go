/** bilibili 平台播放/弹幕能力：随 channels bundle 加载，由数据服务进程登记进 resolver 注册表。 */

import type { PlatformPlayback } from "@amoeba/resolve"
import { biliClient } from "./lib/index.ts"
import { resolveBiliLivePlay } from "./lib/live-play.ts"
import { resolveBiliVideoPlay } from "./lib/video-play.ts"

export const playback: PlatformPlayback = {
  platform: "bilibili",
  resolveVideoPlay: (id, info) => resolveBiliVideoPlay(id, info),
  resolveLivePlay: (id, info) => resolveBiliLivePlay(id, info),
  getDanmaku: (id, opts) => biliClient.getDanmaku(id, { kind: opts?.kind ?? "vod", cookie: opts?.cookie }),
}
