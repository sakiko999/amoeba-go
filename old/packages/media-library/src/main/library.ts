/**
 * 媒体库扫描聚合。领域约定继承旧 Rust core 的 library.rs / naming.rs（见 git 历史）：
 * 库根一级子目录 = 作品；`Season NN`（00=特典）季目录布局优先于扁平布局；
 * 海报查找 poster → folder；全只读，绝不写用户数据。
 */

import { readdir, readFile, stat } from "node:fs/promises";
import { extname, join } from "node:path";
import type {
  EpisodeFile,
  LibraryItem,
  SeasonInfo,
  ShowDetail,
} from "../shared/types";
import { parseEpisodeNfo, parseShowNfo } from "./nfo";

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
const POSTER_NAMES = ["poster.jpg", "poster.png", "folder.jpg", "folder.png"];
const SHOW_NFO_NAMES = ["tvshow.nfo", "movie.nfo"];

const isVideo = (name: string): boolean =>
  VIDEO_EXTS.has(extname(name).slice(1).toLowerCase());

/** `标题 (2011)` → 标题+年份：取最后一个 4 位数字括号组，否则整名当标题 */
export function splitDirName(dirName: string): {
  title: string;
  year?: string;
} {
  const m = dirName.match(/\((\d{4})\)\s*$/);
  if (m?.index !== undefined && m.index > 0) {
    return { title: dirName.slice(0, m.index).trim(), year: m[1] };
  }
  return { title: dirName };
}

async function readText(path: string): Promise<string | undefined> {
  try {
    return await readFile(path, "utf8");
  } catch {
    return undefined;
  }
}

async function findPoster(
  dir: string,
  names: string[],
): Promise<string | undefined> {
  const lower = new Map(names.map((n) => [n.toLowerCase(), n]));
  for (const name of POSTER_NAMES) {
    const hit = lower.get(name);
    if (hit) return join(dir, hit);
  }
  return undefined;
}

/** 只认 `Season NN`（Season 00 = 特典季）；`Specials` 等非标准名不认 */
export function parseSeasonDir(name: string): number | null {
  const m = name.match(/^Season\s+(\d{1,2})$/i);
  return m ? Number(m[1]) : null;
}

/**
 * 集号解析（简化三模式，完整 6 模式见旧 naming.rs）：
 * 1. S01E01（容忍 s1e1 / S01.E01）2. 1x01（排除 1920x1080）3. [NN] 方括号裸集号
 * （排除 [1080p]/[10bit]/[2014]，取最后一个满足约束的组）。0 视为无效。
 */
export function parseEpisodeNumber(fileName: string): number | undefined {
  const sxxexx = fileName.match(/s\d{1,2}[\s._-]?e(\d{1,3})/i);
  if (sxxexx) return Number(sxxexx[1]) || undefined;
  const cross = fileName.match(/(?:^|[^\dx])(\d{1,2})x(\d{1,3})(?:[^\d]|$)/i);
  if (cross && Number(cross[1]) > 0) return Number(cross[2]) || undefined;
  const groups = [...fileName.matchAll(/\[(\d{1,3})(?=[\]_\-. ])/g)];
  const last = groups.at(-1);
  return last ? Number(last[1]) || undefined : undefined;
}

async function scanItem(dir: string, dirName: string): Promise<LibraryItem> {
  const entries = await readdir(dir, { withFileTypes: true });
  const names = entries.map((e) => e.name);

  let nfo: ReturnType<typeof parseShowNfo> = null;
  for (const nfoName of SHOW_NFO_NAMES) {
    const text = await readText(join(dir, nfoName));
    if (text !== undefined) {
      nfo = parseShowNfo(text);
      if (nfo) break;
    }
  }

  const { title, year } = splitDirName(dirName);
  const nfoYear = nfo?.premiered?.match(/^(\d{4})/)?.[1];
  const hasSeasonDir = entries.some(
    (e) => e.isDirectory() && parseSeasonDir(e.name) !== null,
  );

  let videoCount = 0;
  for (const e of entries) {
    if (e.isFile() && isVideo(e.name)) videoCount += 1;
    if (e.isDirectory() && parseSeasonDir(e.name) !== null) {
      const sub = await readdir(join(dir, e.name), {
        withFileTypes: true,
      }).catch(() => []);
      videoCount += sub.filter((s) => s.isFile() && isVideo(s.name)).length;
    }
  }

  return {
    id: dirName,
    title: nfo?.title ?? title,
    year: nfoYear ?? year,
    mediaType: hasSeasonDir ? "tv" : "movie",
    plot: nfo?.plot,
    genres: nfo?.genres ?? [],
    rating: nfo?.rating,
    posterLocal: await findPoster(dir, names),
    dirName,
    dir,
    seasonCount: nfo?.seasonCount,
    episodeCount: nfo?.episodeCount,
    videoCount,
    hasNfo: nfo !== null,
  };
}

async function collectEpisodes(
  dir: string,
  season: number,
): Promise<EpisodeFile[]> {
  const entries = await readdir(dir, { withFileTypes: true }).catch(() => []);
  const lowerNames = new Set(entries.map((e) => e.name.toLowerCase()));
  const episodes: EpisodeFile[] = [];

  for (const e of entries) {
    if (!e.isFile() || !isVideo(e.name)) continue;
    const base = e.name.slice(0, -extname(e.name).length);
    const episode = parseEpisodeNumber(e.name);
    // 每集 NFO 候选：规范集号 S01E01.nfo → 视频同名 .nfo（Kodi 约定）
    let epNfo =
      episode !== undefined
        ? `S${String(season).padStart(2, "0")}E${String(episode).padStart(2, "0")}.nfo`
        : undefined;
    if (epNfo === undefined || !lowerNames.has(epNfo.toLowerCase())) {
      epNfo = `${base}.nfo`;
      if (!lowerNames.has(epNfo.toLowerCase())) epNfo = undefined;
    }
    const meta =
      epNfo !== undefined
        ? parseEpisodeNfo((await readText(join(dir, epNfo))) ?? "")
        : undefined;
    const sizeBytes = await stat(join(dir, e.name))
      .then((s) => s.size)
      .catch(() => undefined);

    episodes.push({
      fileName: e.name,
      path: join(dir, e.name),
      episode,
      season,
      isSpecial: season === 0,
      hasNfo: epNfo !== undefined,
      sizeBytes,
      title: meta?.title,
      aired: meta?.aired,
      runtimeMinutes: meta?.runtimeMinutes,
    });
  }

  // 集号升序，解析不出的排最后（按文件名兜底）
  episodes.sort(
    (a, b) =>
      (a.episode ?? Infinity) - (b.episode ?? Infinity) ||
      a.fileName.localeCompare(b.fileName),
  );
  return episodes;
}

/** 有界并发：大库的 item 扫描是 IO 密集（NFO/海报/stat），串行会明显拖慢首屏 */
async function mapLimit<T, R>(
  items: readonly T[],
  limit: number,
  fn: (item: T) => Promise<R>,
): Promise<R[]> {
  const results: R[] = [];
  let next = 0;
  const workers = Array.from(
    { length: Math.min(limit, items.length) },
    async () => {
      while (next < items.length) {
        const index = next++;
        const item = items[index];
        if (item === undefined) continue;
        results[index] = await fn(item);
      }
    },
  );
  await Promise.all(workers);
  return results;
}

export async function scanLibrary(root: string): Promise<LibraryItem[]> {
  const dirents = await readdir(root, { withFileTypes: true }).catch(() => []);
  const dirs = dirents.filter((d) => d.isDirectory());
  const scanned = await mapLimit(dirs, 8, async (d) => {
    try {
      return await scanItem(join(root, d.name), d.name);
    } catch {
      // 单个作品读失败跳过，不中断整库扫描
      return undefined;
    }
  });
  const items = scanned.filter(
    (item): item is LibraryItem => item !== undefined,
  );
  const yearNum = (i: LibraryItem): number => (i.year ? Number(i.year) : 0);
  items.sort(
    (a, b) =>
      yearNum(b) - yearNum(a) || a.title.localeCompare(b.title, "zh-Hans-CN"),
  );
  return items;
}

export async function showDetail(dir: string): Promise<ShowDetail | null> {
  try {
    const item = await scanItem(
      dir,
      dir.split(/[\\/]/).filter(Boolean).at(-1) ?? dir,
    );
    const entries = await readdir(dir, { withFileTypes: true });
    const seasons = new Map<number, EpisodeFile[]>();

    const seasonDirs = entries
      .filter((e) => e.isDirectory())
      .map((e) => ({ name: e.name, season: parseSeasonDir(e.name) }))
      .filter((s): s is { name: string; season: number } => s.season !== null)
      .sort((a, b) => a.season - b.season);

    if (seasonDirs.length > 0) {
      for (const sd of seasonDirs) {
        seasons.set(
          sd.season,
          await collectEpisodes(join(dir, sd.name), sd.season),
        );
      }
    } else {
      // 扁平布局（字幕组全集包）：视频直接在作品目录下，整体当第一季
      seasons.set(1, await collectEpisodes(dir, 1));
    }

    const sorted: SeasonInfo[] = [...seasons.entries()]
      .sort((a, b) => a[0] - b[0])
      .map(([season, episodes]) => ({ season, episodes }));
    return { item, seasons: sorted };
  } catch {
    return null;
  }
}
