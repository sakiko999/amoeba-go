/**
 * component.json / page.json 读取与校验。
 * - id 同时是 page:// 的 host，Chromium 会小写化 URL，因此只允许小写 DNS label
 * - entry / preload / main 必须是包根内的相对路径；用 realpath 防 ../ 与符号链接穿越
 * - 组件 main v1 只接受 CJS（.js / .cjs）：宿主用 createRequire 加载并清缓存实现热重载
 * - kind=service：必填 main，禁止 entry/preload/color（无界面、不建 tab）
 * - provides/inject：服务名 <domain>.<name> 全局唯一语义，host.* 保留给宿主
 */

import { readFile, realpath, stat } from "node:fs/promises";
import { isAbsolute, join, relative, resolve } from "node:path";
import type {
  PageComponentManifest,
  PageManifest,
  ServiceComponentManifest,
} from "@host/sdk";
import {
  RESERVED_SERVICE_PREFIX as RESERVED,
  SERVICE_NAME_RE as NAME_RE,
} from "@host/sdk";

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

function readServiceName(value: unknown, field: string): string {
  if (typeof value !== "string" || !NAME_RE.test(value)) {
    throw new Error(
      `${field} 服务名非法（应为 <domain>.<name>，小写 DNS label）：${String(value)}`,
    );
  }
  if (value.startsWith(RESERVED)) {
    throw new Error(`${field} 不能使用保留前缀 ${RESERVED}：${value}`);
  }
  return value;
}

function readProvides(raw: unknown): ServiceComponentManifest["provides"] {
  if (!Array.isArray(raw)) throw new Error("provides 必须是数组");
  const seen = new Set<string>();
  return raw.map((item, i) => {
    if (typeof item === "string") {
      // 简写：只给服务名，expose 由运行时 provide 校验（不推荐，安装期不可见）
      const name = readServiceName(item, `provides[${i}]`);
      if (seen.has(name)) throw new Error(`provides 服务名重复：${name}`);
      seen.add(name);
      return { name };
    }
    if (item === null || typeof item !== "object")
      throw new Error(`provides[${i}] 必须是对象或字符串`);
    const rec = item as Record<string, unknown>;
    const name = readServiceName(rec.name, `provides[${i}].name`);
    if (seen.has(name)) throw new Error(`provides 服务名重复：${name}`);
    seen.add(name);
    let expose: string[] | undefined;
    if (rec.expose !== undefined) {
      if (
        !Array.isArray(rec.expose) ||
        rec.expose.some((m) => typeof m !== "string" || m.length === 0)
      ) {
        throw new Error(`provides[${i}].expose 必须是非空字符串数组`);
      }
      expose = rec.expose as string[];
    }
    return { name, expose };
  });
}

function readInject(raw: unknown): string[] {
  if (!Array.isArray(raw)) throw new Error("inject 必须是数组");
  const names = raw.map((item, i) => readServiceName(item, `inject[${i}]`));
  if (new Set(names).size !== names.length)
    throw new Error("inject 服务名重复");
  return names;
}

export async function readPageManifest(root: string): Promise<PageManifest> {
  const realRoot = await realpath(root);
  // component.json 优先；旧 page.json 回退（现有页面包零改动）
  let raw: Record<string, unknown>;
  const componentJson = join(realRoot, "component.json");
  try {
    raw = JSON.parse(await readFile(componentJson, "utf8")) as Record<
      string,
      unknown
    >;
  } catch {
    try {
      raw = JSON.parse(
        await readFile(join(realRoot, "page.json"), "utf8"),
      ) as Record<string, unknown>;
    } catch (err) {
      throw new Error(`component.json/page.json 读取/解析失败：${String(err)}`);
    }
  }

  const id = typeof raw.id === "string" ? raw.id : "";
  if (!ID_RE.test(id)) {
    throw new Error(`page id 非法（只允许小写字母/数字/短横线）：${id}`);
  }
  const name = typeof raw.name === "string" ? raw.name.trim() : "";
  if (name === "") throw new Error("page name 不能为空");

  const version =
    typeof raw.version === "string"
      ? raw.version.trim() || undefined
      : undefined;

  const kind = raw.kind === undefined ? "page" : raw.kind;
  if (kind !== "page" && kind !== "service") {
    throw new Error(`kind 只能是 page | service：${String(raw.kind)}`);
  }

  // 服务声明（两种 kind 都可提供服务；页面组件 = UI + 服务的组合）
  const provides =
    raw.provides === undefined ? undefined : readProvides(raw.provides);
  const inject = raw.inject === undefined ? undefined : readInject(raw.inject);
  if (provides !== undefined && inject !== undefined) {
    const overlap = provides
      .map((p) => p.name)
      .filter((n) => inject.includes(n));
    if (overlap.length > 0) {
      throw new Error(`provides 与 inject 不能重叠：${overlap.join(", ")}`);
    }
  }
  if ((provides?.length ?? 0) > 0 && raw.main === undefined) {
    throw new Error("声明 provides 的组件必须提供 main.js");
  }

  const main = typeof raw.main === "string" ? raw.main : undefined;
  if (main !== undefined) {
    await fileInside(realRoot, realRoot, main, "main", [".js", ".cjs"]);
  }

  if (kind === "service") {
    // 无界面组件：不允许任何页面字段
    for (const field of ["entry", "preload", "color"] as const) {
      if (raw[field] !== undefined) {
        throw new Error(`kind=service 不允许 ${field}`);
      }
    }
    if (main === undefined) throw new Error("kind=service 必须提供 main");
    const manifest: ServiceComponentManifest = {
      id,
      name,
      version,
      kind,
      main,
    };
    if (provides !== undefined) manifest.provides = provides;
    if (inject !== undefined) manifest.inject = inject;
    return manifest;
  }

  const entry = typeof raw.entry === "string" ? raw.entry : "";
  await fileInside(realRoot, realRoot, entry, "entry", [".html"]);

  const preload = typeof raw.preload === "string" ? raw.preload : undefined;
  if (preload !== undefined) {
    await fileInside(realRoot, realRoot, preload, "preload", [".js"]);
  }

  const color =
    typeof raw.color === "string" && /^#[0-9a-f]{6}$/i.test(raw.color)
      ? raw.color
      : undefined;

  const manifest: PageComponentManifest = {
    id,
    name,
    version,
    kind,
    entry,
  };
  if (preload !== undefined) manifest.preload = preload;
  if (color !== undefined) manifest.color = color;
  if (main !== undefined) manifest.main = main;
  if (provides !== undefined) manifest.provides = provides;
  if (inject !== undefined) manifest.inject = inject;
  return manifest;
}
