/** Twitch 平台播放能力 + URL 路由（注册进 api.gateway）。 */

import type { PlatformPlayback } from "@tabs/platform-kit/runtime"
import { resolveTwitchLivePlay } from "./lib/index.ts"
import { isTwitchChannelHost, isTwitchLogin } from "./lib/twitch.ts"

/**
 * 频道页路由：只认频道域（clips./dashboard. 等子域是其它产品）+ 单段路径，
 * login 校验与 normalizeTwitchLogin 同一套字符集/保留字规则（见 lib/twitch.ts）。
 * `/videos/...`、`/<login>/clip/...` 留给后续；clip 页单段路径会被保留字/域检查挡掉。
 */
export const twitchRoutes = [
  {
    test: /(?:^|[./])twitch\.tv\/\S*/i,
    kind: "live",
    extractId(u: string): string | null {
      try {
        const url = new URL(u)
        if (!isTwitchChannelHost(url.hostname)) return null
        const segments = url.pathname.split("/").filter(Boolean)
        if (segments.length !== 1) return null
        let login: string
        try {
          login = decodeURIComponent(segments[0]!).toLowerCase()
        } catch {
          return null
        }
        return isTwitchLogin(login) ? login : null
      } catch {
        return null
      }
    },
  },
] as const

export const playback: PlatformPlayback = {
  platform: "twitch",
  routes: twitchRoutes,
  resolveLivePlay: (id) => resolveTwitchLivePlay(id),
  // 一期无弹幕(聊天需宿主 ws text 帧,二期)。
}
