/**
 * platform 扩展组件的打包辅助：产出 `out/component/`
 *   main.js        —— 组件入口（CJS，node target，宿主 createRequire 加载；
 *                     渠道/lib 实现与 workspace 依赖全部内联进单文件）
 *   component.json
 *   package.json + node_modules —— 仅当组件声明了 npm 依赖（非 workspace）：
 *                     npm 依赖不内联（构建时 external），以 production 依赖形式
 *                     装进产物目录（宿主投影时由 sync-pages 去重到共享层）。
 *                     宿主用 createRequire(main.js) 加载，模块解析从组件目录出发，
 *                     天然命中同目录 node_modules —— 组件可以像普通 npm 包一样
 *                     带依赖（workspace 依赖仍内联，宿主 asar 内没有 workspace 链接）。
 *
 * 依赖运输模式（按声明字段选择，改声明前先读 docs/desktop-release-size.md）：
 * - dependencies（非 workspace）＝ external + 真实文件树安装。给原生扩展（.node）、
 *   运行期动态 require、从自身包目录读数据文件、需要跨组件单例/版本隔离的依赖。
 * - devDependencies ＝ 构建期由 Bun.build tree-shake 内联进 main.js（依赖版本由
 *   仓库 bun.lock 锁定）。纯 JS 静态依赖的默认选择——共享层因此不产生 node_modules。
 *   误把动态依赖放 devDependencies 会在运行时才报 Cannot find module，
 *   由 scripts/pack-components.ts 的内联守卫（无外部字面量 require + 体积预算）兜底。
 */

import { cp, mkdir, rm } from "node:fs/promises";
import { readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";

/** npm 依赖（非 workspace:*）→ external + 产物内安装；workspace 依赖 → 内联。 */
function npmDependencies(pkgRoot: string): Record<string, string> {
  const pkg = JSON.parse(readFileSync(join(pkgRoot, "package.json"), "utf8")) as {
    dependencies?: Record<string, string>;
  };
  const out: Record<string, string> = {};
  for (const [name, spec] of Object.entries(pkg.dependencies ?? {})) {
    if (!spec.startsWith("workspace:")) out[name] = spec;
  }
  return out;
}

async function buildEntry(opts: {
  entry: string;
  outdir: string;
  label: string;
  external: string[];
}): Promise<void> {
  const result = await Bun.build({
    entrypoints: [opts.entry],
    outdir: opts.outdir,
    naming: "main.js",
    target: "node",
    format: "cjs",
    external: opts.external,
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error(`bun build ${opts.label} failed`);
  }
}

export async function buildPlatformComponent(root: string): Promise<void> {
  const pkgRoot = resolve(root);
  const dist = join(pkgRoot, "out", "component");
  const npmDeps = npmDependencies(pkgRoot);
  const external = ["electron", ...Object.keys(npmDeps)];

  await rm(dist, { recursive: true, force: true });
  await mkdir(dist, { recursive: true });
  await cp(join(pkgRoot, "component.json"), join(dist, "component.json"));

  await buildEntry({
    entry: join(pkgRoot, "src", "main.ts"),
    outdir: dist,
    label: "component main",
    external,
  });

  // npm 依赖：以 production 形式装进产物目录（宿主按组件目录解析）。
  if (Object.keys(npmDeps).length > 0) {
    writeFileSync(
      join(dist, "package.json"),
      JSON.stringify({ name: "component-deps", private: true, dependencies: npmDeps }, null, 2) + "\n",
    );
    const proc = Bun.spawn({
      cmd: ["bun", "install", "--production", "--cwd", dist],
      stdout: "inherit",
      stderr: "inherit",
    });
    const code = await proc.exited;
    if (code !== 0) {
      throw new Error(`install component dependencies failed (exit ${code}): ${dist}`);
    }
    console.log(`component deps installed: ${Object.keys(npmDeps).join(", ")}`);
  }

  console.log(`platform component ready: ${dist}`);
}
