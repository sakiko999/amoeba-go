/** TMDB 元数据平台插件：provider 包成 media.tmdb API 注册进 api.gateway。 */

import { defineMediaPlatformExtension } from "@tabs/platform-kit";
import { createTmdbProvider } from "./provider";

// 独立形态（media-manager）从包根静态装配，需要工厂本身
export { createTmdbProvider } from "./provider";

export const { activate, deactivate } = defineMediaPlatformExtension({
  source: "tmdb",
  label: "TMDB",
  createProvider: createTmdbProvider,
});