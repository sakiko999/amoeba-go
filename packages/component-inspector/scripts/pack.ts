/**
 * 把 component-inspector 打成宿主可注册的页面组件包（out/page/）。
 * 共享层：page-vendor manifest 存在时以 PAGE_VENDOR_EXTERNAL=1 触发 external
 * 构建（import map + CSP hash 由 electron.vite.config 的插件在构建期注入）；
 * manifest 缺失（独立构建 / 未跑 pages:prepare）则全内联。
 */

import { existsSync } from "node:fs";
import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const useVendor = existsSync(
  resolve(root, "..", "page-vendor", "out", "dist", "manifest.json"),
);

const build = Bun.spawn({
  cmd: ["bun", "run", "build"],
  cwd: root,
  stdout: "inherit",
  stderr: "inherit",
  env: { ...process.env, PAGE_VENDOR_EXTERNAL: useVendor ? "1" : "0" },
});
if ((await build.exited) !== 0) {
  throw new Error("electron-vite build failed (component-inspector)");
}

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

console.log(
  useVendor
    ? `component-inspector page component ready (vendor layer) → ${dist}`
    : `component-inspector page component ready (全内联) → ${dist}`,
);
