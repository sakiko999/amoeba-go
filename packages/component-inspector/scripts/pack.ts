/**
 * 把 component-inspector 打成宿主可注册的页面组件包（out/page/）。
 * 前置：bun run build（electron-vite 产出 out/renderer 与 out/preload）。
 * 产物：renderer/ + preload/index.js + component.json；无 main.js（无后端状态）。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "page");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "out", "renderer"), join(dist, "renderer"), {
  recursive: true,
});
await cp(
  join(root, "out", "preload", "index.js"),
  join(dist, "preload", "index.js"),
);
await cp(join(root, "component.json"), join(dist, "component.json"));

console.log(`component-inspector page component ready: ${dist}`);
