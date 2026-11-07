/** 斗鱼 平台播放/弹幕能力：随 channels bundle 加载，由数据服务进程登记进 resolver 注册表。 */

import type { PlatformPlayback } from "@amoeba/resolve"
import { douyuClient, douyuResolveStreams } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "douyu",
  resolveLivePlay: (id) => douyuResolveStreams(id),
  getDanmaku: (id) => douyuClient.getDanmaku(id),
}
