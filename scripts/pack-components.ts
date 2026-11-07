/**
 * 打包宿主组件包（顺序执行，任一失败即中止）。
 * 当前组件面（rss 能力暂不启用，源码保留在 packages/ 下）：
 *   基础能力：api-gateway（平台 API 注册中心）/ media-manager（页面）/ media-library
 *   platform 扩展：platform-media-*（媒体元数据渠道）
 * 恢复 rss 域时把 rss-browser / rss-service / rss-client 加回 baseComponents，
 * 并让 extensionComponents 同时匹配 platform-rss-*（目录名现为 platform-<p>，
 * 按 API 归属判断：api.json 存在即 rss 平台）。
 * 产物统一在各自 packages/<pkg>/out/component（或 out/page），由宿主 sync-pages 同步。
 */

import { existsSync } from "node:fs";
import { readdir } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
/** 包名 → pack script 名（media-manager 的页面组件沿用 pack:page） */
const baseComponents: [string, string][] = [
  ["api-gateway", "pack"],
  ["media-manager", "pack:page"],
  ["media-library", "pack"],
];

const packagesDir = join(root, "packages");
const allPackages = (await readdir(packagesDir, { withFileTypes: true }))
  .filter((e) => e.isDirectory())
  .map((e) => e.name);
/** 只打包媒体元数据平台；rss 平台（带 src/channels.ts）暂不启用 */
const extensionComponents = allPackages
  .filter(
    (name) =>
      name.startsWith("platform-") &&
      name !== "platform-kit" &&
      existsSync(join(packagesDir, name, "src", "lib")) === false &&
      existsSync(join(packagesDir, name, "src", "channels.ts")) === false,
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
