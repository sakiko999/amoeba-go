/**
 * 媒体元数据平台（platform-media-*）的统一 provider 契约。
 * 域服务（media-library）与平台插件都只依赖本接口，不互相依赖。
 */

import type {
  EpisodeMetadata,
  MediaMetadata,
  MediaType,
  SearchResult,
} from "./model";

export interface MetadataProvider {
  /** 渠道身份（与 ScrapeSource 一致，如 "tmdb"） */
  readonly source: string;
  search(keyword: string, mediaType: MediaType): Promise<SearchResult[]>;
  fetchMetadata(id: string, mediaType: MediaType): Promise<MediaMetadata>;
  /** tv 每集元数据；渠道不支持返回 [] */
  fetchEpisodes(meta: MediaMetadata, id: string): Promise<EpisodeMetadata[]>;
}
