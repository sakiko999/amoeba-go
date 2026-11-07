/**
 * 刮削应用：拉取元数据（含每集）→ 写 tvshow/movie.nfo + 每集 NFO + 下载 poster.jpg。
 * 写操作只发生在用户明确选择的目录（详情页刮削动作），且只写自己的产物文件。
 * 每集 NFO 跟着视频走（写作品根目录 = 扁平布局约定，Kodi 在视频同目录找同名 NFO）。
 *
 * 元数据渠道经 ProviderRegistry（扩展点）解析：媒体渠道（tmdb/douban/bangumi）
 * 由 media-provider-* 扩展组件注册，未安装时抛可诊断错误。
 */

import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { MediaType, SearchResult } from "../shared/model";
import { episodeNfoFilename, nfoFilename } from "../shared/model";
import type {
  ScrapePreview,
  ScrapeResult,
  ScrapeSource,
} from "../shared/types";
import { writeEpisodeNfo, writeMetadataNfo } from "./nfoWriter";
import { fetchBinary } from "./provider/http";
import type { ProviderRegistry } from "./providers";

export type {
  ScrapePreview,
  ScrapeResult,
  ScrapeSource,
} from "../shared/types";

/** 预览：拉取但不写盘 */
export async function scrapePreview(
  providers: ProviderRegistry,
  source: ScrapeSource,
  id: string,
  mediaType: MediaType,
): Promise<ScrapePreview> {
  const provider = providers.get(source);
  const metadata = await provider.fetchMetadata(id, mediaType);
  const episodes = await provider.fetchEpisodes(metadata, id);
  return { metadata, episodes };
}

/** 应用：写 NFO（含每集）+ 下载海报。海报失败只警告不中断（视频/NFO 已就位） */
export async function scrapeApply(
  providers: ProviderRegistry,
  dir: string,
  source: ScrapeSource,
  id: string,
  mediaType: MediaType,
): Promise<ScrapeResult> {
  const { metadata, episodes } = await scrapePreview(
    providers,
    source,
    id,
    mediaType,
  );

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
  providers: ProviderRegistry,
  source: ScrapeSource,
  keyword: string,
  mediaType: MediaType,
): Promise<SearchResult[]> {
  return providers.get(source).search(keyword, mediaType);
}
