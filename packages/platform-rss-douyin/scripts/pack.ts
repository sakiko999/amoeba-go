/** 构建 platform-rss-douyin 组件包（main.js + channels.js + component.json）。 */

import { join, resolve } from "node:path";
import { buildPlatformComponent } from "@tabs/platform-kit/pack";

const root = resolve(import.meta.dir, "..");
await buildPlatformComponent(root, {
  pluginEntry: join(root, "src", "channels.ts"),
});
