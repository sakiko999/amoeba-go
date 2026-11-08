/**
 * 打包全部宿主组件包（顺序执行，任一失败即中止）。
 * 基础能力组件：api-gateway（平台 API 注册中心）/ media-manager（页面）/
 *              component-inspector（组件依赖页）/ home（首页）/
 *              media-library / browser-proxy（浏览器载具，给数据渠道用）
 * platform 扩展组件：platform-*（平台数据渠道 + 媒体元数据渠道；
 *              两者都只依赖 @tabs/platform-kit / @tabs/platform-contracts，
 *              不依赖 media-library 的域实现）
 * rss 订阅栈（rss-service / rss-client / rss-subscriptions）暂不打包。
 * 产物统一在各自 packages/<pkg>/out/component（或 out/page），由宿主 sync-pages 同步。
 */

import { existsSync } from "node:fs";
import { readdir, stat } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
/** 包名 → pack script 名（media-manager 的页面组件沿用 pack:page） */
const baseComponents: [string, string][] = [
  ["api-gateway", "pack"],
  ["media-manager", "pack:page"],
  ["component-inspector", "pack"],
  ["home", "pack"],
  ["tab-clock", "pack"],
  ["tab-notes", "pack"],
  ["tab-sysinfo", "pack"],
  ["media-library", "pack"],
  ["browser-proxy", "pack"],
];

const packagesDir = join(root, "packages");
const allPackages = (await readdir(packagesDir, { withFileTypes: true }))
  .filter((e) => e.isDirectory())
  .map((e) => e.name);
const extensionComponents = allPackages
  .filter(
    (name) =>
      name.startsWith("platform-") &&
      name !== "platform-kit" &&
      name !== "platform-contracts",
  )
  .sort();

const components: [string, string][] = [
  // 共享层最先构建：页面 pack 依赖它的 manifest 注入 import map
  ["page-vendor", "build"],
  ...baseComponents,
  ...extensionComponents.map((name): [string, string] => [name, "pack"]),
];

for (const [name, script] of components) {
  console.log(`\n=== pack ${name} ===`);
  const proc = Bun.spawn({
    cmd: ["bun", "run", "--cwd", join(packagesDir, name), script],
    cwd: root,
    stdout: "inherit",
    stderr: "inherit",
  });
  const code = await proc.exited;
  if (code !== 0) {
    throw new Error(`pack failed: ${name} (exit ${code})`);
  }
}
// 共享层回归守卫：external 失效时 React 会被重新打进页面包（体积约 10 倍），
// 而功能 smoke 仍然可能全绿；这里用宽松上限（当前最大约 58KB）兜底。
const manifestPath = join(
  packagesDir,
  "page-vendor",
  "out",
  "dist",
  "manifest.json",
);
if (existsSync(manifestPath)) {
  const pagePacks = ["home", "component-inspector", "media-manager"];
  for (const name of pagePacks) {
    const assetsDir = join(packagesDir, name, "out", "page", "renderer", "assets");
    const files = (await readdir(assetsDir)).filter((f) => f.endsWith(".js"));
    let bytes = 0;
    for (const file of files) bytes += (await stat(join(assetsDir, file))).size;
    console.log(`page bundle: ${name.padEnd(20)} ${(bytes / 1024).toFixed(1)} KB`);
    if (bytes > 120 * 1024) {
      throw new Error(
        `${name} 页面 JS 产物 ${(bytes / 1024).toFixed(1)}KB 超过 120KB：` +
          "检查 electron.vite.config 的 external 是否失效",
      );
    }
  }
}

console.log(`\npacked ${components.length} components`);
