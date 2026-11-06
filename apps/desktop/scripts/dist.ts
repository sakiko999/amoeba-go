/**
 * 宿主独立分发打包（asar）：stage = out/ + pages/ + 精简 package.json。
 * 运行 `bun run dist`（内部先 build，会同步 media-manager 页面包）。
 * 产物：release/Tabs Demo-win32-x64/Tabs Demo.exe
 * 验收：该 exe 带 --smoke 运行必须打印 SMOKE OK 并 exit 0。
 */

import { access, cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
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
await cp(join(root, "out"), join(stage, "out"), { recursive: true });

// 内置 runtime 页面包（media-manager 等）；缺了不影响启动，只是没有页面 tab
try {
  await access(join(root, "pages"));
  await cp(join(root, "pages"), join(stage, "pages"), { recursive: true });
} catch {
  console.warn("dist: pages/ 不存在，先跑 bun run pages:prepare");
}

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
console.log(`dist ready: ${appPath}`);
console.log(`verify: "${join(appPath, `${PRODUCT_NAME}.exe`)}" --smoke`);
