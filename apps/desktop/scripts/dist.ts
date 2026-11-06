/**
 * 宿主独立分发打包（asar）：stage = out/ + 精简 package.json。
 * 运行 `bun run dist`（内部先 build，页面包已同步到 out/pages/）。
 * 产物：release/Tabs Demo-win32-x64/Tabs Demo.exe；stage 打包后清理。
 * 验收：该 exe 带 --smoke 运行必须打印 SMOKE OK 并 exit 0。
 */

import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { packager } from "@electron/packager";

const root = resolve(import.meta.dir, "..");
const release = join(root, "release");
const stage = join(release, "app");
const PRODUCT_NAME = "Tabs Demo";

const pkg = JSON.parse(await readFile(join(root, "package.json"), "utf8")) as {
  version?: string;
  devDependencies?: Record<string, string>;
};

const electronVersion = (pkg.devDependencies?.electron ?? "").replace(
  /^[^\d]*/,
  "",
);
if (electronVersion === "") {
  throw new Error("无法从 devDependencies.electron 解析 Electron 版本");
}
const version = pkg.version ?? "0.1.0";

await rm(stage, { recursive: true, force: true });
await mkdir(stage, { recursive: true });
// out/ 已包含 renderer、main、preload 与 out/pages 下的 runtime 页面包
await cp(join(root, "out"), join(stage, "out"), { recursive: true });

// 精简 manifest：packager 只按 main 启动，不需要 workspace / devDependencies
await writeFile(
  join(stage, "package.json"),
  `${JSON.stringify(
    {
      name: "tabs-demo",
      productName: PRODUCT_NAME,
      version,
      description: "Electron multi-tab host with runtime pages",
      author: "tabs-demo",
      main: "out/main/index.js",
    },
    null,
    2,
  )}\n`,
  "utf8",
);

const appPaths = await packager({
  dir: stage,
  name: PRODUCT_NAME,
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
console.log(`verify: "${join(appPath, `${PRODUCT_NAME}.exe`)}" --smoke`);
