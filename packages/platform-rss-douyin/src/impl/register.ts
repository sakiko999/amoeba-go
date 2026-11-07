/**
 * douyin 渠道清单（扩展插件 rss-channel-douyin 的数据面）。
 * 渠道实例本身是无副作用的实现对象；注册动作由消费方执行 ——
 * 基础服务内（registerBuiltinChannels）用本进程注册表，宿主形态由
 * channels.registerPlugin 把插件导出的 channels 数组登记进数据服务。
 */

import type { RssChannel } from "@amoeba/crawler"
import { registerChannel } from "@amoeba/crawler"
import {
  DouyinCategoriesChannel,
  DouyinCategoryChannel,
  DouyinLiveChannel,
  DouyinLiveHotChannel,
  DouyinTrendChannel,
  DouyinUserChannel,
  DouyinUserSearchChannel,
} from "./index.ts"

/** douyin 平台全部渠道实例（插件导出用） */
export const douyinChannels: RssChannel[] = [
  new DouyinLiveChannel(),
  DouyinLiveHotChannel,
  DouyinCategoryChannel,
  DouyinCategoriesChannel,
  DouyinTrendChannel,
  new DouyinUserChannel(),
  new DouyinUserSearchChannel(),
]

export function registerDouyinChannels(): void {
  for (const channel of douyinChannels) registerChannel(channel)
}