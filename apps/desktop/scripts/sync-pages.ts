/**
 * 把 media-manager 的页面包同步到宿主内置页面目录 pages/media-manager。
 * 页面包不存在时只提示并跳过 —— 宿主仍可运行，只是 + 菜单里没有媒体库。
 * 生成页面包：bun run pages:prepare（monorepo 根）。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join, resolve } from "node:path";

const appRoot = resolve(import.meta.dir, "..");
const repoRoot = resolve(appRoot, "../..");
const source = join(repoRoot, "packages", "media-manager", "page-dist");
const dest = join(appRoot, "pages", "media-manager");

if (!existsSync(join(source, "page.json"))) {
  console.log(
    "pages: packages/media-manager/page-dist 不存在，跳过（先运行 bun run pages:prepare）",
  );
  process.exit(0);
}

await rm(dest, { recursive: true, force: true });
await mkdir(join(appRoot, "pages"), { recursive: true });
await cp(source, dest, { recursive: true });
console.log(`pages: synced media-manager → ${dest}`);
