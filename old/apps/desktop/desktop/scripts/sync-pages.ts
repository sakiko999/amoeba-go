/**
 * 把 workspace 包产出的组件包同步到宿主构建目录 out/pages/<manifest.id>
 * （不进 app 根目录，避免 out/ 与 pages/ 两套产物并存；打包时随 out/ 一起进 asar）。
 * 约定：packages/<pkg>/out/<name>/ 下含 component.json 或 page.json 即视为组件包
 * （如 media-manager 的 out/page 与 out/service）。缺失时只提示并跳过 —— 宿主仍可运行。
 * 生成组件包：bun run pages:prepare（monorepo 根）。
 *
 * npm 依赖去重：组件产物各自带 node_modules（自包含、可单独分发）；投影到宿主时
 * 把全部组件的 npm 依赖合并（同名同版本去重）装进 out/pages/node_modules 共享层。
 * Node 的模块解析逐级向上（out/pages/<id>/main.js 的 createRequire 与数据服务进程
 * import <id>/channels.js 都一样），共享层天然被命中——除非组件目录里还有同名包。
 * 版本冲突（同名不同版本）的组件保留自己的完整 node_modules（隔离优先，不共享）。
 */

import { cp, mkdir, readdir, rm, writeFile } from "node:fs/promises";
import { existsSync, readFileSync } from "node:fs";
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

/** 组件产物自带的 npm 依赖清单（pack 写的 package.json；无则空）。 */
function readComponentDeps(dir: string): Record<string, string> {
  const path = join(dir, "package.json");
  if (!existsSync(path)) return {};
  try {
    const raw = JSON.parse(readFileSync(path, "utf8")) as {
      dependencies?: Record<string, string>;
    };
    return raw.dependencies ?? {};
  } catch {
    return {};
  }
}

let synced = 0;
/** 组件 id → 首个来源目录：out/ 里的旧产物若残留同名 id，不能静默覆盖新组件 */
const seen = new Map<string, string>();
/** 共享层依赖：name → version（同名同版本去重） */
const sharedDeps = new Map<string, string>();
/** 版本冲突的组件（保留各自完整 node_modules，不进共享层） */
const conflicted: string[] = [];

// out/pages 是 workspace 组件包的纯投影：先整体清空，避免已删除/改名的旧组件
// （如 rss-channel-* → platform-*）继续被宿主加载
const pagesRoot = join(appRoot, "out", "pages");
await rm(pagesRoot, { recursive: true, force: true });

// 项目级共享层投影：page-vendor 的预构建 ESM（vendor:// 协议的根）。
// 未构建时清掉旧层避免残留半套产物（页面 pack 会同步回退全内联）。
const vendorSource = join(repoRoot, "packages", "page-vendor", "out", "dist");
const vendorRoot = join(appRoot, "out", "vendor");
await rm(vendorRoot, { recursive: true, force: true });
if (existsSync(vendorSource)) {
  await cp(vendorSource, vendorRoot, { recursive: true });
  console.log(`pages: vendor layer synced → ${vendorRoot}`);
} else {
  console.log("pages: page-vendor 未构建，跳过 vendor 层投影");
}

// 已停用的 rss 订阅栈不投影：即使有人手动跑了它们的 pack（out/ 残留），
// 也不能把它们（含已退役的 rss.extend 链路）带进宿主。与
// scripts/pack-components.ts 的打包清单同步维护。
const EXCLUDED_PACKAGES = new Set(["rss-service", "rss-client", "rss-subscriptions"]);

for (const pkg of await readdir(packagesDir, { withFileTypes: true })) {
  if (!pkg.isDirectory() || EXCLUDED_PACKAGES.has(pkg.name)) continue;
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

    const deps = readComponentDeps(source);
    let hasConflict = false;
    for (const [name, version] of Object.entries(deps)) {
      const known = sharedDeps.get(name);
      if (known === undefined) sharedDeps.set(name, version);
      else if (known !== version) hasConflict = true;
    }

    const dest = join(pagesRoot, id);
    await rm(dest, { recursive: true, force: true });
    await mkdir(pagesRoot, { recursive: true });
    await cp(source, dest, {
      recursive: true,
      filter: (src) => {
        // 冲突组件保留自己的 node_modules（隔离）；干净组件剔除（含目录本身），
        // 改用共享层。依赖清单 package.json 也不投影（它只对 pack 产物有意义）。
        if (src === source) return true;
        const rel = src.slice(source.length + 1).replaceAll("\\", "/");
        if (rel === "package.json") return false;
        if (rel === "node_modules" || rel.startsWith("node_modules/")) {
          return hasConflict;
        }
        return true;
      },
    });
    if (hasConflict) conflicted.push(id);
    console.log(
      `pages: synced ${id} (${pkg.name}/${entry.name}) → ${dest}${hasConflict ? "（依赖版本冲突，保留局部 node_modules）" : ""}`,
    );
    synced += 1;
  }
}

// 共享依赖层：同版本只装一份，组件目录向上解析命中
if (sharedDeps.size > 0) {
  const manifest = {
    name: "pages-shared-deps",
    private: true,
    dependencies: Object.fromEntries([...sharedDeps.entries()].sort()),
  };
  await writeFile(
    join(pagesRoot, "package.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
  const proc = Bun.spawn({
    cmd: ["bun", "install", "--production", "--cwd", pagesRoot],
    stdout: "inherit",
    stderr: "inherit",
  });
  const code = await proc.exited;
  if (code !== 0) {
    throw new Error(`install shared page dependencies failed (exit ${code})`);
  }
  console.log(
    `pages: shared deps installed (${[...sharedDeps.keys()].sort().join(", ")})` +
      (conflicted.length > 0 ? `；冲突保留局部：${conflicted.join(", ")}` : ""),
  );
}

if (synced === 0) {
  console.log("pages: 没有可同步的组件包（先运行 bun run pages:prepare）");
}
