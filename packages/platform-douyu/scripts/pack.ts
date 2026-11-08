/** 构建 platform-douyu 组件包（main.js + component.json；渠道实现已内联进 main）。 */

import { resolve } from "node:path";
import { buildPlatformComponent } from "@tabs/platform-kit/pack";

const root = resolve(import.meta.dir, "..");
await buildPlatformComponent(root);
