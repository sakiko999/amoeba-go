/**
 * 把 profile 打成宿主可注册的页面组件包（out/page/，宿主默认页面目录直接指向
 * out/）。vite 已把产物直接写进 out/page/renderer；这里只补 component.json。
 * 共享层：page-vendor manifest 存在时以 PAGE_VENDOR_EXTERNAL=1 触发 external
 * 构建（import map + CSP hash 由 vite 插件在构建期注入）；缺失则全内联。
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
  throw new Error("vite build failed (profile)");
}

const dist = join(root, "out", "page");
await mkdir(dist, { recursive: true });
await copyFile(join(root, "component.json"), join(dist, "component.json"));

console.log(
  useVendor
    ? `profile page component ready (vendor layer) → ${dist}`
    : `profile page component ready (全内联) → ${dist}`,
);
