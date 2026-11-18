/**
 * 把 media-manager 打成宿主可注册的页面组件包（out/page/）。
 * 共享层：page-vendor manifest 存在时以 PAGE_VENDOR_EXTERNAL=1 触发 external
 * 构建（import map + CSP hash 由 electron.vite.config 的插件在构建期注入）；
 * manifest 缺失（独立构建 / 未跑 pages:prepare）则全内联。
 * 产物：renderer/ + preload/index.js + main.js（页面扩展：ctx.require 取
 * media-library 服务组件后绑定渲染侧 IPC）+ component.json（kind=page，
 * inject media.library）。后端服务组件由 packages/media-library 的 pack 产出。
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
  throw new Error("electron-vite build failed (media-manager)");
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

// 页面 main 必须是 CJS：宿主的 createRequire 加载 + require.cache 失效是热重载的基础
// （@tabs/media-library 的源码会被 bundle 进单文件，组件包运行时零依赖）
const result = await Bun.build({
  entrypoints: [join(root, "src", "main", "pageEntry.ts")],
  outdir: dist,
  naming: "main.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!result.success) {
  for (const log of result.logs) console.error(log);
  throw new Error("bun build page main failed");
}

console.log(
  useVendor
    ? `page component ready (vendor layer): ${dist}`
    : `page component ready (vendor manifest 缺失，全内联): ${dist}`,
);
