/**
 * weibo 渠道清单（扩展插件 rss-channel-weibo 的数据面）。
 * 渠道实例本身是无副作用的实现对象；注册动作由消费方执行 ——
 * 基础服务内（registerBuiltinChannels）用本进程注册表，宿主形态由
 * channels.registerPlugin 把插件导出的 channels 数组登记进数据服务。
 */

import type { RssChannel } from "@amoeba/crawler"
import { registerChannel } from "@amoeba/crawler"
import { WeiboHotChannel, WeiboUserChannel, WeiboUserSearchChannel } from "./index.ts"

/** weibo 平台全部渠道实例（插件导出用） */
export const weiboChannels: RssChannel[] = [
  new WeiboUserSearchChannel(),
  new WeiboUserChannel(),
  new WeiboHotChannel(),
]

export function registerWeiboChannels(): void {
  for (const channel of weiboChannels) registerChannel(channel)
}