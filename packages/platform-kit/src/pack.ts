/**
 * platform 扩展组件的打包辅助：产出 `out/component/`
 *   main.js        —— 组件入口（CJS，node target，宿主 createRequire 加载）
 *   <pluginName>   —— 服务进程侧插件 bundle（仅 rss 渠道需要；ESM 单文件，
 *                     bun 服务进程动态 import，平台实现全部 bundle 进来）
 *   component.json
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

export interface PlatformComponentBuildOptions {
  /** 服务进程侧插件入口（rss 渠道传 src/channels.ts；媒体渠道不传） */
  pluginEntry?: string;
  /** 插件产物文件名，默认 channels.js */
  pluginNaming?: string;
}

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

export async function buildPlatformComponent(
  root: string,
  opts: PlatformComponentBuildOptions = {},
): Promise<void> {
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

  if (opts.pluginEntry !== undefined) {
    await buildEntry({
      entry: opts.pluginEntry,
      outdir: dist,
      naming: opts.pluginNaming ?? "channels.js",
      target: "bun",
      format: "esm",
      label: "platform plugin",
    });
  }

  console.log(`platform component ready: ${dist}`);
}
