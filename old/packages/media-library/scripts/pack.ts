/**
 * 把 media-library 打成宿主可注册的服务组件包（out/component/）。
 * 产物：main.js（CJS：serviceEntry，ctx.provide("media.library")）+ component.json
 * （kind=service，provides 声明 expose 面）。无 renderer/preload，无需前置构建。
 * 宿主 scripts/sync-pages.ts 按 manifest 自动同步到 apps/desktop/out/pages/media-library。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "component");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "component.json"), join(dist, "component.json"));

// 服务组件 main 必须是 CJS：宿主的 createRequire 加载 + require.cache 失效是热重载的基础
const result = await Bun.build({
  entrypoints: [join(root, "src", "main", "serviceEntry.ts")],
  outdir: dist,
  naming: "main.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!result.success) {
  for (const log of result.logs) console.error(log);
  throw new Error("bun build service main failed");
}

console.log(`service component ready: ${dist}`);
