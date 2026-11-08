/**
 * 把 component-inspector 打成宿主可注册的页面组件包（out/page/）。
 *
 * 共享层：page-vendor 的 manifest 存在时以 PAGE_VENDOR_EXTERNAL=1 构建
 * （react/@demo/ui external）并注入 import map（vendor://）+ CSP hash；
 * manifest 缺失（独立构建 / 未跑 pages:prepare）则回退全内联。
 */

import { existsSync, readFileSync } from "node:fs";
import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

import {
  injectIntoHtml,
  type VendorManifest,
} from "../../page-vendor/scripts/import-map";

const root = resolve(import.meta.dir, "..");
const manifestPath = resolve(
  root,
  "..",
  "page-vendor",
  "out",
  "dist",
  "manifest.json",
);
const manifest: VendorManifest | undefined = existsSync(manifestPath)
  ? (JSON.parse(readFileSync(manifestPath, "utf8")) as VendorManifest)
  : undefined;

const build = Bun.spawn({
  cmd: ["bun", "run", "build"],
  cwd: root,
  stdout: "inherit",
  stderr: "inherit",
  env: {
    ...process.env,
    PAGE_VENDOR_EXTERNAL: manifest !== undefined ? "1" : "0",
  },
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

if (manifest !== undefined) {
  injectIntoHtml(join(dist, "renderer", "index.html"), manifest);
  console.log(
    `component-inspector page component ready (vendor layer) → ${dist}`,
  );
} else {
  console.log(
    `component-inspector page component ready (vendor manifest 缺失，全内联) → ${dist}`,
  );
}
