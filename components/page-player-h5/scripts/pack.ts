/**
 * 把 player-h5 打成宿主可注册的页面组件包（out/page/，宿主页面目录直接指向
 * out/）。vite 已把产物写进 out/page/renderer；这里只补 component.json。
 * 共享层：page-vendor manifest 存在时以 PAGE_VENDOR_EXTERNAL=1 触发 external 构建。
 */

import { existsSync } from "node:fs";
import { copyFile, mkdir } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const vendorRoot = resolve(root, "..", "..", "web", "page-vendor");
const useVendor = existsSync(join(vendorRoot, "out", "dist", "manifest.json"));

const build = Bun.spawn({
  cmd: ["bun", "run", "build"],
  cwd: root,
  stdout: "inherit",
  stderr: "inherit",
  env: { ...process.env, PAGE_VENDOR_EXTERNAL: useVendor ? "1" : "0" },
});
if ((await build.exited) !== 0) {
  throw new Error("vite build failed (player-h5)");
}

const dist = join(root, "out", "page");
await mkdir(dist, { recursive: true });
await copyFile(join(root, "component.json"), join(dist, "component.json"));

console.log(
  useVendor
    ? `player-h5 page component ready (vendor layer) → ${dist}`
    : `player-h5 page component ready (全内联) → ${dist}`,
);
