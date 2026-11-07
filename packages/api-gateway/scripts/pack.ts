/** 构建 api-gateway 组件包（main.js + component.json）。 */

import { resolve } from "node:path";
import { buildPlatformComponent } from "@tabs/platform-kit/pack";

await buildPlatformComponent(resolve(import.meta.dir, ".."));
