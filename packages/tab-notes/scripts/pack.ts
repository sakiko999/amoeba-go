/**
 * 把 tab-notes 打成宿主页面组件包（out/page/）：
 * renderer/ + component.json（kind=page，无 main/preload，能力由宿主 pageBridge 叠加）。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const build = Bun.spawn({
  cmd: ["bun", "run", "build"],
  cwd: root,
  stdout: "inherit",
  stderr: "inherit",
});
if ((await build.exited) !== 0) {
  throw new Error("electron-vite build failed (tab-notes)");
}

const dist = join(root, "out", "page");
await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "out", "renderer"), join(dist, "renderer"), {
  recursive: true,
});
await cp(join(root, "component.json"), join(dist, "component.json"));

console.log(`tab-notes page component ready → ${dist}`);
