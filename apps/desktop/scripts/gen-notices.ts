/**
 * 生成 apps/desktop/THIRD_PARTY_NOTICES.txt：按「组件 bundle 实际内联的 npm 包」
 * 拼接各包许可证（bundle 后 node_modules 的 LICENSE 不再随包分发，MIT 义务由此承担）。
 * 名单与内联集对应（探针实测）：fxp 的 ESM 闭包 + 经 platform-kit 内联的 ws +
 * page-vendor 共享层（react/react-dom/scheduler）。升级任一依赖后重跑刷新：
 *   bun run apps/desktop/scripts/gen-notices.ts
 */

import { existsSync, readFileSync, readdirSync, realpathSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

const appRoot = resolve(import.meta.dir, "..");
const repoRoot = resolve(appRoot, "../..");

// 直接依赖：[workspace 相对路径, 包名]
const DIRECT: [string, string][] = [
  ["packages/platform-youtube", "ramda"],
  ["packages/platform-youtube", "fast-xml-parser"],
  ["packages/platform-kit", "ws"],
  ["apps/desktop", "react"],
  ["apps/desktop", "react-dom"],
];

// 传递依赖：[锚包名, 目标包名]（anynum 是 strnum、scheduler 是 react-dom 的依赖）
const TRANSITIVE: [string, string][] = [
  ["fast-xml-parser", "@nodable/entities"],
  ["fast-xml-parser", "fast-xml-builder"],
  ["fast-xml-parser", "is-unsafe"],
  ["fast-xml-parser", "path-expression-matcher"],
  ["fast-xml-parser", "strnum"],
  ["fast-xml-parser", "xml-naming"],
  ["strnum", "anynum"],
  ["react-dom", "scheduler"],
];

/** 从 startDir 逐级向上找 node_modules/<name>（bun 的 .bun store 布局也命中）。 */
function findPkgRoot(startDirs: string[], name: string): string {
  for (const start of startDirs) {
    let dir = start;
    for (;;) {
      const candidate = join(dir, "node_modules", name);
      if (existsSync(join(candidate, "package.json"))) return candidate;
      const parent = dirname(dir);
      if (parent === dir) break;
      dir = parent;
    }
  }
  throw new Error(`找不到 ${name} 的安装位置`);
}

interface PkgMeta {
  name?: string;
  version?: string;
  license?: string;
  author?: unknown;
}

function findLicense(root: string, pkg: PkgMeta): string {
  const file = readdirSync(root).find(
    (f) => /^(licen[cs]e|notice|copying)/i.test(f) && !f.endsWith(".md"),
  );
  if (file !== undefined) return readFileSync(join(root, file), "utf8").trim();
  // 上游 tarball 未携带 LICENSE 文件时，从 package.json 元数据合成（并标注）
  const author =
    typeof pkg.author === "string"
      ? pkg.author
      : typeof pkg.author === "object" && pkg.author !== null
        ? String((pkg.author as { name?: string }).name ?? "")
        : "";
  console.warn(`! ${pkg.name ?? root} 无 LICENSE 文件，用 package.json 元数据合成`);
  return [
    `License: ${pkg.license ?? "UNKNOWN"}`,
    author !== "" ? `Copyright (c) ${author}` : "",
    "",
    "(上游 npm tarball 未包含 LICENSE 文件，以上信息来自其 package.json；",
    `完整许可文本见 https://www.npmjs.com/package/${pkg.name ?? ""})`,
  ]
    .filter((line) => line !== "")
    .join("\n");
}

const roots = new Map<string, string>();
for (const [from, name] of DIRECT) {
  roots.set(name, findPkgRoot([join(repoRoot, from)], name));
}
for (const [anchor, name] of TRANSITIVE) {
  const anchorRoot = roots.get(anchor);
  if (anchorRoot === undefined) throw new Error(`锚包 ${anchor} 未解析`);
  // realpath 穿透 workspace junction 进 .bun store，传递依赖与锚包同级
  roots.set(name, findPkgRoot([dirname(realpathSync(anchorRoot))], name));
}

const sections: string[] = [];
for (const name of [...roots.keys()].sort()) {
  const root = roots.get(name);
  if (root === undefined) throw new Error(`${name} 未解析`);
  const pkg = JSON.parse(readFileSync(join(root, "package.json"), "utf8")) as PkgMeta;
  sections.push(
    `${"=".repeat(78)}\n${name} ${pkg.version} — ${pkg.license}\n来源: https://www.npmjs.com/package/${name}\n\n${findLicense(root, pkg)}\n`,
  );
}

const header = `THIRD-PARTY SOFTWARE NOTICES

本应用的组件产物（out/pages、out/vendor）将下列开源包的代码构建期内联进 bundle，
各包的许可证适用于其代码。升级任一依赖后请重跑 apps/desktop/scripts/gen-notices.ts。

`;
await Bun.write(
  join(appRoot, "THIRD_PARTY_NOTICES.txt"),
  header + sections.join("\n"),
);
console.log(`written: THIRD_PARTY_NOTICES.txt (${roots.size} packages)`);
