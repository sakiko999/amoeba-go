/**
 * 独立分发打包（asar）：只打包 out/ 与精简 package.json，不含 node_modules。
 * 运行 `bun run dist`（内部先 electron-vite build）。
 * 产物：release/Media Manager-win32-x64/Media Manager.exe
 * 验收：该 exe 带 --smoke 运行必须打印 SMOKE OK 并 exit 0（asar 内读取已走 Node fs）。
 */

import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { packager } from "@electron/packager";

const root = resolve(import.meta.dir, "..");
const release = join(root, "release");
const stage = join(release, "app");

const pkg = JSON.parse(await readFile(join(root, "package.json"), "utf8")) as {
  version?: string;
  productName?: string;
  description?: string;
  author?: string;
  devDependencies?: Record<string, string>;
};

const electronVersion = (pkg.devDependencies?.electron ?? "").replace(
  /^[^\d]*/,
  "",
);
if (electronVersion === "") {
  throw new Error("无法从 devDependencies.electron 解析 Electron 版本");
}
const version = pkg.version ?? "0.0.0";
const productName = pkg.productName ?? "Media Manager";

await rm(stage, { recursive: true, force: true });
await mkdir(stage, { recursive: true });
await cp(join(root, "out"), join(stage, "out"), { recursive: true });
// out/page 是给宿主用的 runtime 页面包，独立应用不需要，别塞进 asar
await rm(join(stage, "out", "page"), { recursive: true, force: true });

// 精简 manifest：packager 只按 main 启动，不需要 devDependencies / workspace 依赖
await writeFile(
  join(stage, "package.json"),
  `${JSON.stringify(
    {
      name: "media-manager",
      productName,
      version,
      description: pkg.description,
      author: pkg.author,
      main: "out/main/index.js",
    },
    null,
    2,
  )}\n`,
  "utf8",
);

const appPaths = await packager({
  dir: stage,
  name: productName,
  platform: "win32",
  arch: "x64",
  out: release,
  overwrite: true,
  asar: true,
  prune: false,
  electronVersion,
  appVersion: version,
});

const appPath = appPaths[0];
if (appPath === undefined) throw new Error("packager 未返回产物路径");
// stage 只是 packager 的输入，产物已生成，删掉避免 release 里两套 app
await rm(stage, { recursive: true, force: true });
console.log(`dist ready: ${appPath}`);
console.log(`verify: "${join(appPath, `${productName}.exe`)}" --smoke`);
