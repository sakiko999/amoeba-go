/** TMDB 元数据平台插件：provider 包成 tmdb API 注册进 api.gateway。 */

import { defineMediaPlatformExtension } from "@tabs/platform-kit";
import { createTmdbProvider } from "./provider";

export const { activate, deactivate } = defineMediaPlatformExtension({
  source: "tmdb",
  label: "TMDB",
  createProvider: createTmdbProvider,
});