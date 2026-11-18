/**
 * 把 rss-service 打成宿主可注册的服务组件包（out/component/）。
 *
 * 产物（全部在本仓库构建，不依赖 tauri-rss 的外部产物）：
 * - main.js     组件入口（CJS，node target：spawn 数据服务 + READY 协议）
 * - service.js  数据服务本体（bun target 单文件 ESM，源自 runtime/main.ts；
 *               runtime/ 与 packages/amoeba/* 是从 tauri-rss 移植的源码）
 * - component.json
 *
 * 运行时需要 bun（PATH 或 AMOEBA_BUN）—— 这是数据服务当前的临时形态；
 * 迁移到进程内（node:sqlite 等）是后续工作，见 CLAUDE.md。
 * 可用 AMOEBA_SERVICE_JS 指定外部 service.js 覆盖组件内副本（调试用）。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "component");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "component.json"), join(dist, "component.json"));

// 1) 组件入口（Node/Electron 主进程侧）
const mainBuild = await Bun.build({
  entrypoints: [join(root, "src", "entry.ts")],
  outdir: dist,
  naming: "main.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!mainBuild.success) {
  for (const log of mainBuild.logs) console.error(log);
  throw new Error("bun build rss-service main failed");
}

// 2) 数据服务本体（Bun 进程侧；与 tauri-rss 的 service:build 同一构建方式）
const serviceBuild = await Bun.build({
  entrypoints: [join(root, "runtime", "main.ts")],
  outdir: dist,
  naming: "service.js",
  target: "bun",
});
if (!serviceBuild.success) {
  for (const log of serviceBuild.logs) console.error(log);
  throw new Error("bun build rss-service service bundle failed");
}

console.log(`rss service component ready: ${dist}`);
