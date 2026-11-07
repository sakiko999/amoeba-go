/** 构建 rss-channel-bili 组件包（main.js + channels.js + component.json）。 */

import { resolve } from "node:path";
import { buildChannelExtension } from "@tabs/rss-channel-kit/pack";

await buildChannelExtension(resolve(import.meta.dir, ".."));