/**
 * 宿主独立分发打包（asar）：stage = out/ + 精简 package.json。
 * 运行 `bun run dist`（内部先 build，页面包已同步到 out/pages/）。
 * 产物：release/Tabs Demo-win32-x64/Tabs Demo.exe；stage 打包后清理。
 * 打包后裁剪 Electron 运行时（locales 白名单 + 未用到的可选 GPU 组件），
 * 依据与验证见 docs/desktop-release-size.md。
 * 验收：该 exe 带 --smoke 运行必须打印 SMOKE OK 并 exit 0。
 */

import {
  cp,
  mkdir,
  readdir,
  readFile,
  rm,
  stat,
  writeFile,
} from "node:fs/promises";
import { existsSync } from "node:fs";
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

const LOCALES_KEEP = new Set(["zh-CN.pak", "en-US.pak"]);
// GPU 进程启动即预载这些文件、失败非致命（gpu_init.cc）；实际使用方是 WebGPU /
// Skia Graphite(D3D12)——本应用未用，WebGL 软件回退走 ANGLE D3D11 + WARP（系统
// 组件）。升级 Electron 时复核：Chromium 已计划在 Windows 默认开启 Graphite，
// 届时删 dxcompiler 会让渲染静默退回 D3D11，需要 WebGPU 时从列表移除并重打包。
const RUNTIME_DROP_FILES = [
  "dxcompiler.dll",
  "vk_swiftshader.dll",
  "vk_swiftshader_icd.json",
  "vulkan-1.dll",
];

/** packager 的 ignore 只作用于 app 目录，运行时文件必须打包后删（返回释放字节数）。 */
async function pruneRuntime(appPath: string): Promise<number> {
  let freed = 0;
  const locales = join(appPath, "locales");
  for (const entry of await readdir(locales, { withFileTypes: true })) {
    // 只删 *.pak：将来 locales/ 若出现辅助文件不误伤
    if (!entry.isFile() || !entry.name.endsWith(".pak")) continue;
    if (LOCALES_KEEP.has(entry.name)) continue;
    const path = join(locales, entry.name);
    freed += (await stat(path)).size;
    await rm(path);
  }
  // en-US.pak 是 ResourceBundle 的硬性兜底（缺失时 crash_on_failure 直接崩）
  if (!existsSync(join(locales, "en-US.pak"))) {
    throw new Error("prune 后 locales/en-US.pak 缺失，产物将无法启动");
  }
  for (const name of RUNTIME_DROP_FILES) {
    const path = join(appPath, name);
    const info = await stat(path).catch(() => undefined);
    if (info === undefined) continue;
    freed += info.size;
    await rm(path);
  }
  return freed;
}

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
const freedBytes = await pruneRuntime(appPath);
// 内联依赖的许可声明放 release 根（与 LICENSES.chromium.html 并列，便于审查）
await cp(
  join(root, "THIRD_PARTY_NOTICES.txt"),
  join(appPath, "THIRD_PARTY_NOTICES.txt"),
);
console.log(`runtime pruned: ${(freedBytes / 1048576).toFixed(2)} MB freed`);
// stage 只是 packager 的输入，产物已生成，删掉避免 release 里两套 app
await rm(stage, { recursive: true, force: true });
console.log(`dist ready: ${appPath}`);
console.log(`verify: "${join(appPath, `${PRODUCT_NAME}.exe`)}" --smoke`);
