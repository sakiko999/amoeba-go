/** Bangumi 元数据平台插件：provider 包成 bangumi API 注册进 api.gateway。 */

import { defineMediaPlatformExtension } from "@tabs/platform-kit";
import { createBangumiProvider } from "./provider";

export const { activate, deactivate } = defineMediaPlatformExtension({
  source: "bangumi",
  label: "Bangumi",
  createProvider: createBangumiProvider,
});