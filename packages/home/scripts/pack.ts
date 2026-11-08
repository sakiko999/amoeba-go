/**
 * 把 home 打成宿主可注册的页面组件包（out/page/）。
 * 产物：index.html + renderer.js（浏览器 ESM）+ preload.js（CJS）+ component.json。
 * 无 main.js：页面没有后端状态，数据来自宿主只读 IPC 与 NAV_CHANNELS.open。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "page");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "component.json"), join(dist, "component.json"));
await cp(join(root, "src", "index.html"), join(dist, "index.html"));

const renderer = await Bun.build({
  entrypoints: [join(root, "src", "renderer.ts")],
  outdir: dist,
  naming: "renderer.js",
  target: "browser",
  format: "esm",
});
if (!renderer.success) {
  for (const log of renderer.logs) console.error(log);
  throw new Error("bun build home renderer failed");
}

const preload = await Bun.build({
  entrypoints: [join(root, "src", "preload.ts")],
  outdir: dist,
  naming: "preload.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!preload.success) {
  for (const log of preload.logs) console.error(log);
  throw new Error("bun build home preload failed");
}

console.log(`home page ready: ${dist}`);
