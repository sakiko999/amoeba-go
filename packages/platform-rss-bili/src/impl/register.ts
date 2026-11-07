/**
 * bili 渠道清单（扩展插件 platform-rss-bili 的数据面）。
 * 渠道实例本身是无副作用的实现对象；注册动作由消费方执行 ——
 * 基础服务内（registerBuiltinChannels）用本进程注册表，宿主形态由
 * channels.registerPlugin 把插件导出的 channels 数组登记进数据服务。
 */

import type { RssChannel } from "@amoeba/crawler"
import { registerChannel } from "@amoeba/crawler"
import {
  BiliAnchorChannel,
  BiliDynamicChannel,
  BiliFavlistChannel,
  BiliLiveAreaChannel,
  BiliLiveAreasChannel,
  BiliLiveChannel,
  BiliLiveHotChannel,
  BiliPopularChannel,
  BiliPreciousChannel,
  BiliRankingChannel,
  BiliSeasonChannel,
  BiliSeriesChannel,
  BiliSquareChannel,
  BiliUserChannel,
  BiliUserVideoChannel,
  BiliWeeklyChannel,
} from "./index.ts"

/** bili 平台全部渠道实例（插件导出用） */
export const biliChannels: RssChannel[] = [
  BiliSquareChannel,
  BiliPopularChannel,
  BiliPreciousChannel,
  BiliRankingChannel,
  BiliWeeklyChannel,
  BiliUserVideoChannel,
  new BiliLiveChannel(),
  BiliLiveHotChannel,
  BiliLiveAreasChannel,
  BiliLiveAreaChannel,
  new BiliSeasonChannel(),
  new BiliSeriesChannel(),
  BiliFavlistChannel,
  new BiliDynamicChannel(),
  BiliAnchorChannel,
  new BiliUserChannel(),
]

export function registerBiliChannels(): void {
  for (const channel of biliChannels) registerChannel(channel)
}