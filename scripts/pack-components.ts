/**
 * 打包全部宿主组件包（顺序执行，任一失败即中止）。
 * 基础能力组件：api-gateway（平台 API 注册中心）/ media-manager（页面）/
 *              media-library / rss-browser（浏览器载具，给数据渠道用）
 * platform 扩展组件：platform-*（平台数据渠道 + 媒体元数据渠道；
 *              两者都只依赖 @tabs/platform-kit / @tabs/platform-contracts，
 *              不依赖 media-library 的域实现）
 * rss 订阅栈（rss-service / rss-client / rss-subscriptions）暂不打包。
 * 产物统一在各自 packages/<pkg>/out/component（或 out/page），由宿主 sync-pages 同步。
 */

import { readdir } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
/** 包名 → pack script 名（media-manager 的页面组件沿用 pack:page） */
const baseComponents: [string, string][] = [
  ["api-gateway", "pack"],
  ["media-manager", "pack:page"],
  ["media-library", "pack"],
  ["rss-browser", "pack"],
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
console.log(`\npacked ${components.length} components`);
