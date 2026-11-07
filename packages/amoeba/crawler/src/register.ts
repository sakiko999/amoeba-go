/**
 * 全量渠道注册（基础 + 全部平台）——仅独立/测试场景使用。
 *
 * ⚠️ 宿主形态不要 import 本文件：它会把所有平台渠道（连同平台 client）拉进
 * bundle。平台渠道在宿主里是扩展插件（rss-channel-* 组件），经
 * `channels.registerPlugin` 运行时登记；基础 bundle 只经 register-base.ts 注册。
 *
 * 新增平台渠道：在对应 `channels/<platform>/register.ts` 里登记，并在此汇总。
 */

import { registerBaseChannels } from "./register-base.ts"
import { registerAppleChannels } from "./channels/apple/register.ts"
import { registerBiliChannels } from "./channels/bili/register.ts"
import { registerDouyinChannels } from "./channels/douyin/register.ts"
import { registerDouyuChannels } from "./channels/douyu/register.ts"
import { registerHuyaChannels } from "./channels/huya/register.ts"
import { registerTwitchChannels } from "./channels/twitch/register.ts"
import { registerWeiboChannels } from "./channels/weibo/register.ts"
import { registerXhsChannels } from "./channels/xhs/register.ts"
import { registerYoutubeChannels } from "./channels/youtube/register.ts"

/** 注册所有内置 channel（基础 + 全平台）。无守卫——由调用方保证幂等。 */
export function registerBuiltinChannels(): void {
  registerBaseChannels()
  registerAppleChannels()
  registerBiliChannels()
  registerDouyinChannels()
  registerDouyuChannels()
  registerHuyaChannels()
  registerTwitchChannels()
  registerWeiboChannels()
  registerXhsChannels()
  registerYoutubeChannels()
}
