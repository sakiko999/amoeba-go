/**
 * huya 渠道清单（扩展插件 platform-huya 的数据面）。
 * 渠道实例本身是无副作用的实现对象；注册动作由消费方执行 ——
 * 基础服务内（registerBuiltinChannels）用本进程注册表，宿主形态由
 * channels.registerPlugin 把插件导出的 channels 数组登记进数据服务。
 */

import type { RssChannel } from "@amoeba/crawler"
import { registerChannel } from "@amoeba/crawler"
import {
  HuyaAnchorChannel,
  HuyaCategoriesChannel,
  HuyaCategoryChannel,
  HuyaLiveChannel,
  HuyaLiveHotChannel,
} from "./index.ts"

/** huya 平台全部渠道实例（插件导出用） */
export const huyaChannels: RssChannel[] = [
  new HuyaLiveChannel(),
  HuyaLiveHotChannel,
  HuyaCategoryChannel,
  new HuyaCategoriesChannel(),
  HuyaAnchorChannel,
]

export function registerHuyaChannels(): void {
  for (const channel of huyaChannels) registerChannel(channel)
}