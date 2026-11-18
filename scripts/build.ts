/**
 * 集中构建编排：chrome → 页面组件（自动发现）→ go。
 *
 * 页面发现规则与 main.go 的 discoverPages 同源：components 下每个子目录的
 * component.json 里 kind=page 者各自 pack。新增页面组件 = 建目录 + manifest，零登记。
 *
 * 用法：bun run scripts/build.ts [dev|build|fe|chrome|pages]（默认 build）
 *   dev    构建前端后 go run .
 *   build  构建前端后 go build → tmp/mygo-host/mygo-host.exe
 *   fe     只构建前端（dlv 调试的 preLaunchTask：Go 侧由调试器自己编译启动）
 *   chrome / pages  只跑对应阶段
 */

import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");

function discoverPages(): { name: string; dir: string }[] {
  const base = join(root, "components");
  return readdirSync(base, { withFileTypes: true })
    .filter((e) => e.isDirectory())
    .map((e) => ({ name: e.name, dir: join(base, e.name) }))
    .filter(({ dir }) => {
      const manifest = join(dir, "component.json");
      if (!existsSync(manifest)) return false;
      const kind = (JSON.parse(readFileSync(manifest, "utf8")) as { kind?: string }).kind;
      return kind === "page";
    });
}

async function run(cmd: string[], cwd = root): Promise<void> {
  const p = Bun.spawn({ cmd, cwd, stdout: "inherit", stderr: "inherit" });
  if ((await p.exited) !== 0) {
    throw new Error(`构建步骤失败: ${cmd.join(" ")}`);
  }
}

const mode = process.argv[2] ?? "build";
const pages = discoverPages();
console.log(`页面组件(${pages.length}): ${pages.map((p) => p.name).join(", ") || "无"}`);

if (mode === "chrome" || mode === "fe" || mode === "dev" || mode === "build") {
  await run(["bun", "run", "--cwd", "web/chrome", "build"]);
}
if (mode === "pages" || mode === "fe" || mode === "dev" || mode === "build") {
  for (const { dir } of pages) {
    await run(["bun", "run", "--cwd", dir, "pack"]);
  }
}
if (mode === "dev") {
  await run(["go", "run", "."]);
} else if (mode === "build") {
  await run(["go", "build", "-o", "tmp/mygo-host/mygo-host.exe", "."]);
} else if (mode !== "chrome" && mode !== "pages" && mode !== "fe") {
  throw new Error(`未知子命令: ${mode}（dev|build|fe|chrome|pages）`);
}
