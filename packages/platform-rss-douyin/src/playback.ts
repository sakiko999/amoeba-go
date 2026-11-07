/** 抖音 平台播放/弹幕能力：随 channels bundle 加载，由数据服务进程登记进 resolver 注册表。 */

import type { PlatformPlayback } from "@amoeba/resolve"
import { douyinClient, douyinResolveStreams } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "douyin",
  resolveLivePlay: (id) => douyinResolveStreams(id),
  // cookie 未传时 client 内部走 douyinCookie()(provider 感知,登录态可用)。
  getDanmaku: (id) => douyinClient.getDanmaku(id),
}
