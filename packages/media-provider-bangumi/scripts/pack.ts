/**
 * 把 media-provider-bangumi 打成宿主可注册的服务组件包（out/component/）。
 * 产物：main.js（CJS）+ component.json；零运行时依赖。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "component");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "component.json"), join(dist, "component.json"));

const result = await Bun.build({
  entrypoints: [join(root, "src", "main.ts")],
  outdir: dist,
  naming: "main.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!result.success) {
  for (const log of result.logs) console.error(log);
  throw new Error("bun build media-provider-bangumi main failed");
}

console.log(`media provider (bangumi) component ready: ${dist}`);
