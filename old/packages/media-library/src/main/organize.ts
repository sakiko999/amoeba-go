/**
 * 媒体库目录整理（移植自旧 core 的 organize/mod.rs）—— 本应用唯一改动用户文件的模块，
 * 安全设计是硬性要求：
 * - plan 与 apply 分离：前端必须先展示计划、用户确认后才 apply
 * - 源目录是符号链接/junction → 直接拒绝（血换来的闸门：曾因 junction + apply 丢过原片，
 *   不要为了「方便测试」移除）
 * - 用 rename 移动；跨设备 EXDEV → copy + delete，复制失败保留源
 * - 目标已存在 → 跳过并报告，绝不覆盖；计划内多个源指向同一目标同样全部跳过
 * - 特典按文件名排序统一编号（不用 CM 数字 —— 实测两套 CM 号完全重复会交错）
 */

import { existsSync } from "node:fs";
import {
  copyFile,
  lstat,
  mkdir,
  readdir,
  rename,
  unlink,
  writeFile,
} from "node:fs/promises";
import { dirname, join } from "node:path";
import type { MediaMetadata } from "../shared/model";
import { metadataYear, nfoFilename } from "../shared/model";
import type { OrganizePlan, PlanItem, PlanStatus } from "../shared/types";
import { writeMetadataNfo } from "./nfoWriter";
import {
  canonicalFilename,
  canonicalSpecialFilename,
  parseKind,
  seasonDir,
  showDir,
} from "./naming";

const VIDEO_EXTS = new Set([
  "mkv",
  "mp4",
  "avi",
  "mov",
  "wmv",
  "flv",
  "m4v",
  "ts",
  "m2ts",
  "rmvb",
  "webm",
  "iso",
]);

const isVideo = (name: string): boolean =>
  VIDEO_EXTS.has(name.slice(name.lastIndexOf(".") + 1).toLowerCase());

export interface ApplyOptions {
  /** 模板里的 {path} 替换为视频原始路径（逐段百分号编码）；不传则不生成 */
  strmTemplate?: string;
}

/** 扫描 sourceDir 生成计划。**不触碰任何文件**。 */
export async function planOrganize(
  sourceDir: string,
  libraryRoot: string,
  meta: MediaMetadata,
): Promise<OrganizePlan> {
  const stat = await lstat(sourceDir).catch(() => undefined);
  if (stat === undefined) throw new Error(`源目录不存在：${sourceDir}`);
  if (stat.isSymbolicLink()) {
    throw new Error(
      `拒绝操作符号链接目录：${sourceDir}\n指向链接会把链接目标的数据搬走或删除，这是不可逆的。请直接对真实路径操作。`,
    );
  }

  const showRoot = join(libraryRoot, showDir(meta.title, metadataYear(meta)));
  const nfoPath = join(showRoot, nfoFilename(meta.mediaType));
  const entries = await readdir(sourceDir, { withFileTypes: true });

  const items: PlanItem[] = [];
  const specials: { path: string; name: string }[] = [];

  for (const entry of entries) {
    // 不递归：假定待整理文件都在同一层，递归会带来「移动后目录变空」等边界情况
    if (!entry.isFile() || !isVideo(entry.name)) continue;
    const from = join(sourceDir, entry.name);
    const kind = parseKind(entry.name);
    if (kind === undefined) {
      items.push({
        from,
        status: "skip",
        reason: `无法从文件名识别季集号：${entry.name}（支持 S01E01 / 1x01 / 第N集 / EP01 / [NN] / 「 - NN 」）`,
      });
      continue;
    }
    if (kind.kind === "special") {
      specials.push({ path: from, name: entry.name });
      continue;
    }
    items.push({
      from,
      to: join(
        showRoot,
        seasonDir(kind.se.season),
        canonicalFilename(entry.name, kind.se),
      ),
      status: "move",
    });
  }

  // 特典统一顺次编号（字节序排序保证可复现）
  specials.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  specials.forEach((sp, i) => {
    items.push({
      from: sp.path,
      to: join(
        showRoot,
        seasonDir(0),
        canonicalSpecialFilename(sp.name, i + 1),
      ),
      status: "move",
    });
  });

  resolveTargetConflicts(items);
  items.sort((a, b) => (a.from < b.from ? -1 : a.from > b.from ? 1 : 0));

  const count = (s: PlanStatus): number =>
    items.filter((i) => i.status === s).length;
  return {
    showRoot,
    nfoPath,
    items,
    toMove: count("move"),
    skipped: count("skip"),
    alreadyInPlace: count("skip_already_in_place"),
  };
}

/** 冲突消解：安全性最关键的一步 —— 任何漏网冲突都会在 apply 时变成覆盖或中途失败 */
function resolveTargetConflicts(items: PlanItem[]): void {
  const claims = new Map<string, number[]>();
  items.forEach((item, i) => {
    if (item.to === undefined) return;
    const list = claims.get(item.to) ?? [];
    list.push(i);
    claims.set(item.to, list);
  });

  items.forEach((item) => {
    if (item.to === undefined || item.status !== "move") return;
    if (item.from === item.to) {
      item.status = "skip_already_in_place";
      return;
    }
    const claimants = claims.get(item.to) ?? [];
    if (claimants.length > 1) {
      item.status = "skip";
      item.reason = `计划内多个文件指向同一目标，跳过：${item.to}`;
      return;
    }
    if (existsSync(item.to)) {
      item.status = "skip";
      item.reason = `目标已存在，拒绝覆盖：${item.to}`;
    }
  });
}

/** 跨设备移动：rename 失败且 EXDEV 时 copy + delete；复制失败保留源 */
async function moveFile(from: string, to: string): Promise<void> {
  await mkdir(dirname(to), { recursive: true });
  if (existsSync(to)) throw new Error(`目标已存在，拒绝覆盖：${to}`);
  try {
    await rename(from, to);
    return;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "EXDEV") throw err;
  }
  await copyFile(from, to);
  // 复制成功后才删源；删除失败只警告（源与目标并存，可人工清理）
  await unlink(from).catch((err: unknown) => {
    console.warn(`跨设备移动后删除源文件失败（目标已就位）：${from}`, err);
  });
}

/** 逐段百分号编码（实测含中日文、[]、空格、& 的路径都要能进 URL） */
function encodePath(path: string): string {
  return path.split(/[\\/]/).map(encodeURIComponent).join("/");
}

/** 执行计划（调用方必须已获得用户明确授权）。单条失败不中断，逐条返回结果。 */
export async function applyOrganize(
  plan: OrganizePlan,
  meta: MediaMetadata,
  opts: ApplyOptions = {},
): Promise<PlanItem[]> {
  await mkdir(plan.showRoot, { recursive: true });
  const results: PlanItem[] = [];

  for (const item of plan.items) {
    if (item.status !== "move" || item.to === undefined) {
      results.push(item);
      continue;
    }
    try {
      await moveFile(item.from, item.to);
      if (opts.strmTemplate !== undefined && opts.strmTemplate !== "") {
        const strmPath = item.to.replace(/\.[^.\\/]+$/, ".strm");
        const url = opts.strmTemplate.replaceAll(
          "{path}",
          encodePath(item.from),
        );
        // STRM 是附加产物，写失败不该让整理失败
        await writeFile(strmPath, `${url}\n`, "utf8").catch((err: unknown) => {
          console.warn(`STRM 写入失败（视频已就位）：${strmPath}`, err);
        });
      }
      results.push({ ...item });
    } catch (err) {
      results.push({
        ...item,
        status: "skip",
        reason: `移动失败：${err instanceof Error ? err.message : String(err)}`,
      });
    }
  }

  await writeFile(plan.nfoPath, writeMetadataNfo(meta), "utf8");
  return results;
}
