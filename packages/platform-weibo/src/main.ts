/** 微博 平台插件：向 rss.extend 登记 channels.js（渠道实现随包构建，运行时加载进数据服务）。 */

import { defineRssPlatformExtension } from "@tabs/platform-kit";

export const { activate, deactivate } = defineRssPlatformExtension();
