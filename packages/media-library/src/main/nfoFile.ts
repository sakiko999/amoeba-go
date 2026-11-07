/**
 * NFO 文件 IO + 目录级元数据推断。
 * 编辑 / 刮削 / 整理都走这里，保证「读到的」与「写回的」是同一套 MediaMetadata。
 */

import { readFile, readdir, writeFile } from "node:fs/promises";
import { basename, join } from "node:path";
import type { MediaMetadata, MediaType } from "../shared/model";
import { newMetadata, nfoFilename } from "../shared/model";
import { parseMetadataNfo } from "./nfo";
import { writeMetadataNfo } from "./nfoWriter";
import { parseSeasonDir, splitDirName } from "./library";

const CANDIDATES: { name: string; mediaType: MediaType }[] = [
  { name: "tvshow.nfo", mediaType: "tv" },
  { name: "movie.nfo", mediaType: "movie" },
];

/** 读作品目录内的 NFO；不存在或没有 title 时返回 undefined */
export async function readMetadataNfo(
  dir: string,
): Promise<MediaMetadata | undefined> {
  for (const c of CANDIDATES) {
    try {
      const xml = await readFile(join(dir, c.name), "utf8");
      const meta = parseMetadataNfo(xml, c.mediaType);
      if (meta !== null) return meta;
    } catch {
      // 尝试下一个候选
    }
  }
  return undefined;
}

/** 写作品级 NFO（文件名由 mediaType 决定），返回绝对路径 */
export async function writeMetadataNfoFile(
  dir: string,
  meta: MediaMetadata,
): Promise<string> {
  const path = join(dir, nfoFilename(meta.mediaType));
  await writeFile(path, writeMetadataNfo(meta), "utf8");
  return path;
}

/** 无 NFO 时按目录名 + 是否含 Season 目录推断（整理目标命名用） */
export async function inferMetadata(dir: string): Promise<MediaMetadata> {
  const { title, year } = splitDirName(basename(dir));
  const entries = await readdir(dir, { withFileTypes: true }).catch(() => []);
  const isTv = entries.some(
    (e) => e.isDirectory() && parseSeasonDir(e.name) !== null,
  );
  const m = newMetadata(isTv ? "tv" : "movie", title);
  if (year !== undefined) m.premiered = year;
  return m;
}

/** NFO 优先，否则按目录推断 */
export async function metadataForDir(dir: string): Promise<MediaMetadata> {
  return (await readMetadataNfo(dir)) ?? (await inferMetadata(dir));
}
