/** 抖音 平台播放/弹幕能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { douyinClient, douyinResolveStreams } from "./lib/index.ts"

export const playback: PlatformPlayback = {
  platform: "douyin",
  routes: [
    {
      test: /(?:^|[./])live\.douyin\.com\/(\S+)/,
      kind: "live",
      extractId: (u) => {
        try {
          const last = new URL(u).pathname.replace(/\/+$/, "").split("/").pop()
          return last && last !== "-" ? last : null
        } catch {
          return null
        }
      },
    },
  ],
  resolveLivePlay: (id) => douyinResolveStreams(id),
  // cookie 未传时 client 内部走 douyinCookie()(provider 感知,登录态可用)。
  getDanmaku: (id) => douyinClient.getDanmaku(id),
}
