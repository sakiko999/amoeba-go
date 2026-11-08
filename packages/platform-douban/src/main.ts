/** 豆瓣 元数据平台插件：provider 包成 douban API 注册进 api.gateway。 */

import { defineMediaPlatformExtension } from "@tabs/platform-kit";
import { createDoubanProvider } from "./provider";

export const { activate, deactivate } = defineMediaPlatformExtension({
  source: "douban",
  label: "豆瓣",
  createProvider: createDoubanProvider,
});