/** Twitch 平台播放/弹幕能力：随 channels bundle 加载，由数据服务进程登记进 resolver 注册表。 */

import type { PlatformPlayback } from "@amoeba/resolve"
import { resolveTwitchLivePlay } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "twitch",
  resolveLivePlay: (id) => resolveTwitchLivePlay(id),
  // 一期无弹幕(聊天需宿主 ws text 帧,二期)。
}
