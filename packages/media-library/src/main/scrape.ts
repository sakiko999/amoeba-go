/**
 * 刮削应用：拉取元数据（含每集）→ 写 tvshow/movie.nfo + 每集 NFO + 下载 poster.jpg。
 * 写操作只发生在用户明确选择的目录（详情页刮削动作），且只写自己的产物文件。
 * 每集 NFO 跟着视频走（写作品根目录 = 扁平布局约定，Kodi 在视频同目录找同名 NFO）。
 */

import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import type {
  EpisodeMetadata,
  MediaMetadata,
  MediaType,
  SearchResult,
} from "../shared/model";
import { episodeNfoFilename, nfoFilename } from "../shared/model";
import type {
  ScrapePreview,
  ScrapeResult,
  ScrapeSource,
} from "../shared/types";
import { writeEpisodeNfo, writeMetadataNfo } from "./nfoWriter";
import {
  fetchBangumi,
  fetchBangumiEpisodes,
  searchBangumi,
} from "./provider/bangumi";
import { fetchDouban } from "./provider/douban";
import { fetchBinary } from "./provider/http";
import { fetchTmdb, fetchTmdbEpisodes, searchTmdb } from "./provider/tmdb";

export type {
  ScrapePreview,
  ScrapeResult,
  ScrapeSource,
} from "../shared/types";

async function fetchMetadata(
  source: ScrapeSource,
  id: string,
  mediaType: MediaType,
): Promise<MediaMetadata> {
  switch (source) {
    case "bangumi":
      return fetchBangumi(id, mediaType);
    case "tmdb":
      return fetchTmdb(id, mediaType);
    case "douban":
      return fetchDouban(id);
  }
}

async function fetchEpisodes(
  source: ScrapeSource,
  meta: MediaMetadata,
  id: string,
): Promise<EpisodeMetadata[]> {
  if (meta.mediaType !== "tv") return [];
  switch (source) {
    case "bangumi":
      // 一次请求全集
      return fetchBangumiEpisodes(id);
    case "tmdb":
      // 按季逐个请求；失败只警告
      return meta.seasonCount !== undefined && meta.seasonCount > 0
        ? fetchTmdbEpisodes(id, meta.seasonCount)
        : [];
    case "douban":
      // 豆瓣没有每集接口（旧项目同样不支持）
      return [];
  }
}

/** 预览：拉取但不写盘 */
export async function scrapePreview(
  source: ScrapeSource,
  id: string,
  mediaType: MediaType,
): Promise<ScrapePreview> {
  const metadata = await fetchMetadata(source, id, mediaType);
  const episodes = await fetchEpisodes(source, metadata, id);
  return { metadata, episodes };
}

/** 应用：写 NFO（含每集）+ 下载海报。海报失败只警告不中断（视频/NFO 已就位） */
export async function scrapeApply(
  dir: string,
  source: ScrapeSource,
  id: string,
  mediaType: MediaType,
): Promise<ScrapeResult> {
  const { metadata, episodes } = await scrapePreview(source, id, mediaType);

  const nfoPath = join(dir, nfoFilename(mediaType));
  await writeFile(nfoPath, writeMetadataNfo(metadata), "utf8");

  let episodeNfos = 0;
  for (const ep of episodes) {
    await writeFile(
      join(dir, episodeNfoFilename(ep)),
      writeEpisodeNfo(ep),
      "utf8",
    );
    episodeNfos += 1;
  }

  let posterPath: string | undefined;
  if (metadata.posterUrl !== undefined && metadata.posterUrl !== "") {
    try {
      const buf = await fetchBinary(metadata.posterUrl);
      posterPath = join(dir, "poster.jpg");
      await writeFile(posterPath, buf);
    } catch (err) {
      console.warn(`poster download failed for ${id}:`, err);
    }
  }

  return { nfoPath, posterPath, episodeNfos };
}

export async function searchMetadata(
  source: ScrapeSource,
  keyword: string,
  mediaType: MediaType,
): Promise<SearchResult[]> {
  switch (source) {
    case "bangumi":
      return searchBangumi(keyword);
    case "tmdb":
      return searchTmdb(keyword, mediaType);
    case "douban":
      throw new Error("[douban] 豆瓣没有稳定的搜索 API，只能按 ID 刮削");
  }
}
