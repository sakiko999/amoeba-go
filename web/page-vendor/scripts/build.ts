/**
 * 构建共享层（out/dist）：预构建 ESM + 内容 hash + manifest（Bun.build）。
 *
 * 关键点：
 * - 每个 entry 单独构建：**捆绑自己的包、external 其余共享裸名**（把 entry 自己
 *   的包也 external 会产出空 facade → import map 自引用死循环）。
 * - react-dom 的根与 /client 是同一个 entry（同一实例）；scheduler 等内联。
 * - CJS 包（react/react-dom）的 `export *` 经 bundler 只剩 default —— 命名导出
 *   必须按运行时实际键面**生成**（out/gen），与所装版本永远一致。
 * - CJS 依赖对 external 的 require 由 Bun 转成静态 ESM import（rolldown-vite 会
 *   留运行时 require shim，浏览器不可用）。
 *
 * manifest.json 记录 entry→file、包版本与 baseUrl；页面 pack 与宿主构建读它注入
 * import map。版本漂移时（不同页不同版本）应重跑本构建对齐。
 */

import { createRequire } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "dist");
const gen = join(root, "out", "gen");
const req = createRequire(join(root, "package.json"));

await rm(join(root, "out"), { recursive: true, force: true });
await mkdir(gen, { recursive: true });

/** 生成命名导出绑定的 entry（CJS 包必须显式枚举；`export *` 会丢命名面） */
function writeGeneratedEntry(
  file: string,
  modules: Array<{ specifier: string; alias: string }>,
): string {
  const first = modules[0];
  if (first === undefined) throw new Error(`生成 entry 需要至少一个模块：${file}`);
  const keys = new Map<string, string>(); // 导出名 → 模块别名
  for (const { specifier, alias } of modules) {
    for (const key of Object.keys(req(specifier))) {
      if (key === "default" || keys.has(key)) continue;
      keys.set(key, alias);
    }
  }
  const lines = [
    ...modules.map(({ specifier, alias }) => `import * as ${alias} from ${JSON.stringify(specifier)};`),
    `export default ${first.alias};`,
    ...[...keys.entries()].map(
      ([key, alias]) => `export const ${key} = ${alias}.${key};`,
    ),
  ];
  const path = join(gen, file);
  void writeFile(path, `${lines.join("\n")}\n`);
  return path;
}

const reactEntry = writeGeneratedEntry("react.ts", [
  { specifier: "react", alias: "__react" },
]);
const reactDomEntry = writeGeneratedEntry("react-dom.ts", [
  { specifier: "react-dom", alias: "__rd" },
  { specifier: "react-dom/client", alias: "__rc" },
]);

const ENTRIES: Array<{ name: string; input: string; external: string[] }> = [
  { name: "react", input: reactEntry, external: [] },
  {
    name: "jsx-runtime",
    input: join(root, "entries", "jsx-runtime.ts"),
    external: [],
  },
  { name: "react-dom", input: reactDomEntry, external: ["react"] },
  {
    name: "demo-ui",
    input: join(root, "entries", "demo-ui.ts"),
    external: ["react", "react/jsx-runtime", "react-dom", "react-dom/client"],
  },
  {
    name: "platform-icons",
    input: join(root, "entries", "platform-icons.ts"),
    external: ["react", "react/jsx-runtime"],
  },
];

const files: Record<string, string> = {};
for (const { name, input, external } of ENTRIES) {
  const result = await Bun.build({
    entrypoints: [input],
    outdir: dist,
    external,
    format: "esm",
    target: "browser",
    minify: true,
    jsx: { runtime: "automatic" },
    sourcemap: "none",
    naming: "[name]-[hash].[ext]",
    // CJS 依赖按生产分支静态求值；浏览器里没有 process.env
    define: { "process.env.NODE_ENV": '"production"' },
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`vendor entry 构建失败：${name}`);
  }
  const entryOut = result.outputs.find((o) => o.kind === "entry-point");
  if (entryOut === undefined) throw new Error(`vendor entry 无产物：${name}`);
  files[name] = basename(entryOut.path);
}

if (Object.keys(files).length !== ENTRIES.length) {
  throw new Error(`vendor 构建产物缺 entry：${JSON.stringify(files)}`);
}

const pkgVersion = (name: string): string => {
  let dir = dirname(req.resolve(name));
  while (!existsSync(join(dir, "package.json"))) dir = dirname(dir);
  return (JSON.parse(readFileSync(join(dir, "package.json"), "utf8")) as { version: string }).version;
};

const manifest = {
  baseUrl: "vendor://app/",
  versions: {
    react: pkgVersion("react"),
    "react-dom": pkgVersion("react-dom"),
    "@demo/ui": pkgVersion("@demo/ui"),
    "platform-icons": "workspace",
  },
  entries: files,
};
await writeFile(join(dist, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);

for (const [name, file] of Object.entries(files)) {
  const text = readFileSync(join(dist, file), "utf8");
  const requireLeak = /\brequire\s*\(/.test(text);
  console.log(
    `vendor: ${name.padEnd(14)} ${file}  ${(text.length / 1024).toFixed(0)} KB${requireLeak ? "  ⚠ 含 require()" : ""}`,
  );
  if (requireLeak) throw new Error(`vendor entry 输出含 require()：${name}`);
}
console.log(`vendor layer ready: ${dist}`);
