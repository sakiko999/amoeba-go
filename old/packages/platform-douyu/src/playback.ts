/** 斗鱼 平台播放/弹幕能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { douyuClient, douyuResolveStreams } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "douyu",
  routes: [
    { test: /(?:^|[./])douyu\.com\/(\d+)/, kind: "live", extractId: (u) => u.match(/douyu\.com\/(\d+)/)?.[1] ?? null },
  ],
  resolveLivePlay: (id) => douyuResolveStreams(id),
  getDanmaku: (id) => douyuClient.getDanmaku(id),
}
