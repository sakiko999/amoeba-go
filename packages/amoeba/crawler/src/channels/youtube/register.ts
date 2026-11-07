/**
 * youtube 渠道清单（扩展插件 rss-channel-youtube 的数据面）。
 * 渠道实例本身是无副作用的实现对象；注册动作由消费方执行 ——
 * 基础服务内（registerBuiltinChannels）用本进程注册表，宿主形态由
 * channels.registerPlugin 把插件导出的 channels 数组登记进数据服务。
 */

import type { RssChannel } from "../../index.ts"
import { registerChannel } from "../../index.ts"
import {
  YoutubeChannel,
  YoutubeLiveCategoriesChannel,
  YoutubeLiveCategoryChannel,
  YoutubeLiveChannel,
  YoutubeLiveHotChannel,
  YoutubePlaylistChannel,
  YoutubeUserChannel,
} from "./index.ts"

/** youtube 平台全部渠道实例（插件导出用） */
export const youtubeChannels: RssChannel[] = [
  new YoutubeChannel(),
  YoutubePlaylistChannel,
  YoutubeLiveChannel,
  YoutubeLiveHotChannel,
  YoutubeLiveCategoryChannel,
  YoutubeLiveCategoriesChannel,
  YoutubeUserChannel,
]

export function registerYoutubeChannels(): void {
  for (const channel of youtubeChannels) registerChannel(channel)
}