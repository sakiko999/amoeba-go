/**
 * 收集发布版内嵌资源：页面组件与 vendor 共享层复制到 embedded/，并写入内容摘要。
 * 由 `bun run build` 在 Go 编译前调用；开发构建不调用，二进制因此没有 embedded/。
 */

import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { cp, mkdir, rm, writeFile } from "node:fs/promises";
import { join, relative, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const embedded = join(root, "tmp", "embedded");
const vendor = join(root, "web", "page-vendor", "out", "dist");

if (!existsSync(join(vendor, "manifest.json"))) {
  throw new Error(`缺少 vendor 共享层: ${vendor}（先构建 web/page-vendor）`);
}

await rm(embedded, { recursive: true, force: true });
await mkdir(join(embedded, "pages"), { recursive: true });

const pages: string[] = [];
for (const entry of readdirSync(join(root, "components"), { withFileTypes: true })) {
  if (!entry.isDirectory()) continue;
  const out = join(root, "components", entry.name, "out");
  if (!existsSync(join(out, "page", "component.json"))) continue;
  await cp(out, join(embedded, "pages", entry.name), { recursive: true });
  pages.push(entry.name);
}
if (pages.length === 0) throw new Error("没有已构建的页面组件（先运行 bun run build 的页面阶段）");

await cp(vendor, join(embedded, "vendor"), { recursive: true });

const hash = createHash("sha256");
const walk = (dir: string): string[] =>
  readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    return entry.isDirectory() ? walk(path) : [path];
  });
for (const file of walk(embedded).sort()) {
  hash.update(relative(embedded, file).replaceAll("\\", "/"));
  hash.update(readFileSync(file));
}

await writeFile(join(embedded, "digest"), `${hash.digest("hex")}\n`);
console.log(`embedded assets: ${pages.join(", ")}`);
