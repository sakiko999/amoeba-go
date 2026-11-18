/** 虎牙 平台播放/弹幕能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { huyaClient, resolveHuyaLivePlay } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "huya",
  routes: [
    { test: /(?:^|[./])huya\.com\/(\d+)/, kind: "live", extractId: (u) => u.match(/huya\.com\/(\d+)/)?.[1] ?? null },
  ],
  resolveLivePlay: (id) => resolveHuyaLivePlay(id),
  getDanmaku: (id) => huyaClient.getDanmaku(id),
}
