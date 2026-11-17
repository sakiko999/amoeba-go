/**
 * 媒体请求的 Referer/Origin/UA 规则表（移植自 packages/amoeba/resolve/src/media-referrer.ts，
 * 规则与注释同源；图片图床规则未迁移——本页不直出远程 <img>）。
 *
 * 原项目（tauri-rss）的分工：JS 层设不了 Referer/UA/Origin（forbidden header），
 * 由 Rust 在 WebView2 网络层按同一张表改写请求头。mygo 侧请求直接从 Go 发出
 * （@mygo-plugins/fetch），任何 header 都可设置——规则表因此下沉到请求出口
 * （hostFetch.ts），不再需要网络层拦截。
 */

/** 视频直链的防盗链 Referer（命中即带站内 Referer；未命中可空 Referer 拉取）。 */
export const VIDEO_REFERRERS: Array<[string, string]> = [
  ["xhscdn.com", "https://www.xiaohongshu.com/"],
  ["weibocdn.com", "https://weibo.com/"],
  ["sinaimg.cn", "https://weibo.com/"],
  ["douyinvod.com", "https://www.douyin.com/"],
]

/** XHR/fetch 的 Origin 改写规则（Twitch playlist 域拒页面源 Origin）。值不带尾斜杠。 */
export const XHR_ORIGIN_RULES: Array<[string, string]> = [
  ["playlist.ttvnw.net", "https://www.twitch.tv"],
]

/** huya HLS CDN 只认这条播放器 UA（浏览器 UA → 403；与平台 play 的 playUA 同源）。 */
export const HUYA_HLS_UA =
  "HYSDK(Windows, 30000002)_APP(pc_exe&7060000&official)_SDK(trans&2.32.3.5646)"

/** XHR/fetch 的 UA 改写规则（host 后缀 → 指定 UA）。 */
export const XHR_UA_RULES: Array<[string, string]> = [["hls.huya.com", HUYA_HLS_UA]]

function hostOf(url: string): string | undefined {
  return url.match(/^https?:\/\/([^/]+)/)?.[1]
}

function ruleFor(rules: Array<[string, string]>, url: string): string | undefined {
  const host = hostOf(url)
  if (!host) return undefined
  return rules.find(([suffix]) => host.includes(suffix))?.[1]
}

export function videoReferrerFor(url: string): string | undefined {
  return ruleFor(VIDEO_REFERRERS, url)
}

export function xhrOriginFor(url: string): string | undefined {
  return ruleFor(XHR_ORIGIN_RULES, url)
}

export function xhrUserAgentFor(url: string): string | undefined {
  return ruleFor(XHR_UA_RULES, url)
}
