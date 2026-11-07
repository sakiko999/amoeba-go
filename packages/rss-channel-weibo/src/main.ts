/** 微博渠道扩展组件：向 rss.extend 登记 channels.js（渠道实现随包构建，运行时加载进数据服务）。 */

import { defineChannelExtension } from "@tabs/rss-channel-kit";

export const { activate, deactivate } = defineChannelExtension();