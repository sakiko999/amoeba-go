/**
 * 把 workspace 包产出的组件包同步到宿主构建目录 out/pages/<manifest.id>
 * （不进 app 根目录，避免 out/ 与 pages/ 两套产物并存；打包时随 out/ 一起进 asar）。
 * 约定：packages/<pkg>/out/<name>/ 下含 component.json 或 page.json 即视为组件包
 * （如 media-manager 的 out/page 与 out/service）。缺失时只提示并跳过 —— 宿主仍可运行。
 * 生成组件包：bun run pages:prepare（monorepo 根）。
 */

import { cp, mkdir, readdir, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join, resolve } from "node:path";

const appRoot = resolve(import.meta.dir, "..");
const repoRoot = resolve(appRoot, "../..");
const packagesDir = join(repoRoot, "packages");

async function readManifestId(dir: string): Promise<string | undefined> {
  for (const file of ["component.json", "page.json"]) {
    const path = join(dir, file);
    if (!existsSync(path)) continue;
    try {
      const raw = JSON.parse(await Bun.file(path).text()) as {
        id?: unknown;
      };
      if (typeof raw.id === "string" && raw.id.length > 0) return raw.id;
    } catch {
      console.warn(`pages: ${path} 解析失败，跳过该目录`);
      return undefined;
    }
  }
  return undefined;
}

let synced = 0;
/** 组件 id → 首个来源目录：out/ 里的旧产物若残留同名 id，不能静默覆盖新组件 */
const seen = new Map<string, string>();
// out/pages 是 workspace 组件包的纯投影：先整体清空，避免已删除/改名的旧组件
// （如 rss-channel-* → platform-rss-*）继续被宿主加载
const pagesRoot = join(appRoot, "out", "pages");
await rm(pagesRoot, { recursive: true, force: true });
for (const pkg of await readdir(packagesDir, { withFileTypes: true })) {
  if (!pkg.isDirectory()) continue;
  const outDir = join(packagesDir, pkg.name, "out");
  const entries = await readdir(outDir, { withFileTypes: true }).catch(
    () => undefined,
  );
  if (entries === undefined) continue;
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const source = join(outDir, entry.name);
    const id = await readManifestId(source);
    if (id === undefined) continue;
    const existing = seen.get(id);
    if (existing !== undefined) {
      console.warn(`pages: id 冲突，跳过 ${source}（已用 ${existing}）`);
      continue;
    }
    seen.set(id, source);
    const dest = join(pagesRoot, id);
    await rm(dest, { recursive: true, force: true });
    await mkdir(pagesRoot, { recursive: true });
    await cp(source, dest, { recursive: true });
    console.log(`pages: synced ${id} (${pkg.name}/${entry.name}) → ${dest}`);
    synced += 1;
  }
}
if (synced === 0) {
  console.log("pages: 没有可同步的组件包（先运行 bun run pages:prepare）");
}
