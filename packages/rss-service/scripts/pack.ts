/**
 * 把 rss-service 打成宿主可注册的服务组件包（out/component/）。
 * 产物：main.js（CJS 组件入口）+ component.json + service.js（若 tauri-rss 的
 * 数据服务构建产物存在则复制进来，组件自包含；AMOEBA_SERVICE_JS 可覆盖）。
 * 运行时还需要 bun（PATH 或 AMOEBA_BUN）。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const dist = join(root, "out", "component");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(join(root, "component.json"), join(dist, "component.json"));

const result = await Bun.build({
  entrypoints: [join(root, "src", "entry.ts")],
  outdir: dist,
  naming: "main.js",
  target: "node",
  format: "cjs",
  external: ["electron"],
});
if (!result.success) {
  for (const log of result.logs) console.error(log);
  throw new Error("bun build rss-service main failed");
}

// 数据服务自包含 bundle（~1.2MB）：来自 tauri-rss 的 service:build 产物
const sourceCandidates = [
  process.env.AMOEBA_SERVICE_JS,
  "X:\\tauri-rss\\dist\\service\\service.js",
].filter((p): p is string => typeof p === "string" && p.length > 0);
const serviceJs = sourceCandidates.find((p) => existsSync(p));
if (serviceJs) {
  await cp(serviceJs, join(dist, "service.js"));
  console.log(`service bundle: ${serviceJs}`);
} else {
  console.log(
    "service bundle: 未找到（pack 后组件依赖 AMOEBA_SERVICE_JS 运行时指定）",
  );
}

console.log(`rss service component ready: ${dist}`);
