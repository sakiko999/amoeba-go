/**
 * 渠道扩展组件的打包辅助：产出一个组件包目录
 *   main.js      —— 组件入口（CJS，node target，宿主的 createRequire 加载）
 *   channels.js  —— 渠道插件（ESM 单文件，bun 服务进程动态 import；
 *                  crawler 渠道实现与平台 client 全部 bundle 进来，自包含）
 *   component.json
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

async function buildEntry(opts: {
  entry: string;
  outdir: string;
  naming: string;
  target: "node" | "bun";
  format: "cjs" | "esm";
  label: string;
}): Promise<void> {
  const result = await Bun.build({
    entrypoints: [opts.entry],
    outdir: opts.outdir,
    naming: opts.naming,
    target: opts.target,
    format: opts.format,
    external: ["electron"],
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`bun build ${opts.label} failed`);
  }
}

export async function buildChannelExtension(root: string): Promise<void> {
  const pkgRoot = resolve(root);
  const dist = join(pkgRoot, "out", "component");

  await rm(dist, { recursive: true, force: true });
  await mkdir(dist, { recursive: true });
  await cp(join(pkgRoot, "component.json"), join(dist, "component.json"));

  await buildEntry({
    entry: join(pkgRoot, "src", "main.ts"),
    outdir: dist,
    naming: "main.js",
    target: "node",
    format: "cjs",
    label: "component main",
  });
  await buildEntry({
    entry: join(pkgRoot, "src", "channels.ts"),
    outdir: dist,
    naming: "channels.js",
    target: "bun",
    format: "esm",
    label: "channels plugin",
  });

  console.log(`rss channel component ready: ${dist}`);
}
