/**
 * page.json 读取与校验。
 * - id 同时是 page:// 的 host，Chromium 会小写化 URL，因此只允许小写 DNS label
 * - entry / preload / main 必须是包根内的相对路径；用 realpath 防 ../ 与符号链接穿越
 * - 页面 main v1 只接受 CJS（.js / .cjs）：宿主用 createRequire 加载并清缓存实现热重载
 */

import { readFile, realpath, stat } from "node:fs/promises";
import { isAbsolute, join, relative, resolve } from "node:path";
import type { PageManifest } from "@host/sdk";

const ID_RE = /^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$/;

/** 严格包含关系（child 不能等于 root，也不能逃出 root） */
export function isInside(root: string, child: string): boolean {
  const rel = relative(root, child);
  return rel !== "" && !rel.startsWith("..") && !isAbsolute(rel);
}

async function fileInside(
  realRoot: string,
  root: string,
  relPath: string,
  field: string,
  exts: string[],
): Promise<void> {
  if (relPath.trim() === "" || isAbsolute(relPath)) {
    throw new Error(`${field} 必须是包根内的相对路径`);
  }
  const file = resolve(root, relPath);
  if (!isInside(root, file))
    throw new Error(`${field} 越出页面目录：${relPath}`);
  const real = await realpath(file).catch(() => {
    throw new Error(`${field} 不存在：${relPath}`);
  });
  if (!isInside(realRoot, real)) {
    throw new Error(`${field} realpath 越出页面目录：${relPath}`);
  }
  if (!(await stat(real)).isFile()) {
    throw new Error(`${field} 不是文件：${relPath}`);
  }
  if (!exts.includes(real.slice(real.lastIndexOf(".")).toLowerCase())) {
    throw new Error(`${field} 扩展名必须是 ${exts.join(" / ")}：${relPath}`);
  }
}

export async function readPageManifest(root: string): Promise<PageManifest> {
  const realRoot = await realpath(root);
  let raw: Record<string, unknown>;
  try {
    raw = JSON.parse(
      await readFile(join(realRoot, "page.json"), "utf8"),
    ) as Record<string, unknown>;
  } catch (err) {
    throw new Error(`page.json 读取/解析失败：${String(err)}`);
  }

  const id = typeof raw.id === "string" ? raw.id : "";
  if (!ID_RE.test(id)) {
    throw new Error(`page id 非法（只允许小写字母/数字/短横线）：${id}`);
  }
  const name = typeof raw.name === "string" ? raw.name.trim() : "";
  if (name === "") throw new Error("page name 不能为空");

  const entry = typeof raw.entry === "string" ? raw.entry : "";
  await fileInside(realRoot, realRoot, entry, "entry", [".html"]);

  const preload = typeof raw.preload === "string" ? raw.preload : undefined;
  if (preload !== undefined) {
    await fileInside(realRoot, realRoot, preload, "preload", [".js"]);
  }
  const main = typeof raw.main === "string" ? raw.main : undefined;
  if (main !== undefined) {
    await fileInside(realRoot, realRoot, main, "main", [".js", ".cjs"]);
  }

  const color =
    typeof raw.color === "string" && /^#[0-9a-f]{6}$/i.test(raw.color)
      ? raw.color
      : undefined;
  const version =
    typeof raw.version === "string"
      ? raw.version.trim() || undefined
      : undefined;

  return { id, name, version, color, entry, preload, main };
}
