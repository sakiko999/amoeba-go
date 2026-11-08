/**
 * twitch 平台组件入口：渠道与播放/弹幕能力直接注册到 api.gateway。
 * 数据接口在宿主主进程内执行（appHost 由 platform-kit runtime 安装）。
 */

import { definePlatformExtension } from "@tabs/platform-kit/runtime/extension";
import { channels, playback } from "./channels.ts";

export const { activate, deactivate } = definePlatformExtension({
  channels,
  playback,
});
