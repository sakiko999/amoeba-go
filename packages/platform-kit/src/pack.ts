/**
 * platform 扩展组件的打包辅助：产出 `out/component/`
 *   main.js        —— 组件入口（CJS，node target，宿主 createRequire 加载）
 *   <pluginName>   —— 服务进程侧插件 bundle（仅 rss 渠道需要；ESM 单文件，
 *                     bun 服务进程动态 import，平台实现全部 bundle 进来）
 *   api.json       —— rss 平台渠道的静态描述（构建时 import 插件 bundle 提取，
 *                     组件 main 据此向 api.gateway 注册 `rss.<平台>` 元数据 API）
 *   component.json
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

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

    // 渠道实例是纯描述对象（构造无网络副作用）：直接 import 产物提取静态描述，
    // 组件 main 在宿主主进程据此向 api.gateway 注册元数据 API。
    const manifest = JSON.parse(
      readFileSync(join(pkgRoot, "component.json"), "utf8"),
    ) as { id: string; name: string };
    const namespace = manifest.id.replace(/^platform-/, "");
    const mod = (await import(
      pathToFileURL(join(dist, opts.pluginNaming ?? "channels.js")).href
    )) as { channels?: Array<Record<string, unknown>> };
    const channels = (mod.channels ?? []).map((c) => ({
      key: c.key,
      name: c.name,
      kind: c.kind,
      ...(c.passive === true ? { passive: true } : {}),
      ...(Array.isArray(c.sourceInfoTpl)
        ? { sourceInfoTpl: c.sourceInfoTpl }
        : {}),
    }));
    if (channels.length === 0) {
      throw new Error("plugin bundle 未导出非空 channels 数组，无法生成 api.json");
    }
    writeFileSync(
      join(dist, "api.json"),
      JSON.stringify({ namespace, label: manifest.name, channels }, null, 2),
    );
  }

  console.log(`platform component ready: ${dist}`);
}
