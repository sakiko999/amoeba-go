/** TMDB 元数据平台插件：向 media.providers 注册 provider（同进程直接传对象）。 */

import { defineMediaPlatformExtension } from "@tabs/platform-kit";
import { createTmdbProvider } from "./provider";

// 独立形态（media-manager）从包根静态装配，需要工厂本身
export { createTmdbProvider } from "./provider";

export const { activate, deactivate } = defineMediaPlatformExtension(createTmdbProvider);
