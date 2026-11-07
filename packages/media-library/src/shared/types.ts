/**
 * main / preload / renderer 三方共享的契约类型（media-manager 内部）。
 * 字段语义继承旧 Rust core 的 library.rs（完整版见该仓库 git 历史）。
 * 刮削 / 整理的类型也在这里，main 侧实现直接复用，避免前后端各写一份。
 */

import type { HostState } from "@host/sdk";
import type { EpisodeMetadata, MediaMetadata, SearchResult } from "./model";

export type {
  Actor,
  EpisodeMetadata,
  MediaMetadata,
  Rating,
  SearchResult,
  UniqueId,
} from "./model";

export type MediaKind = "tv" | "movie";

export interface LibraryItem {
  /** 库内唯一 id（= 目录名） */
  id: string;
  title: string;
  originalTitle?: string;
  /** 4 位年份字符串；无 NFO 时从目录名 `标题 (2011)` 解析，可能缺失 */
  year?: string;
  mediaType: MediaKind;
  plot?: string;
  genres: string[];
  /** 0-10，取 NFO 第一个 rating */
  rating?: number;
  /** 海报绝对路径（poster/folder 命名约定） */
  posterLocal?: string;
  dirName: string;
  /** 作品目录绝对路径 */
  dir: string;
  seasonCount?: number;
  episodeCount?: number;
  videoCount: number;
  hasNfo: boolean;
}

export interface EpisodeFile {
  fileName: string;
  path: string;
  /** 从文件名解析出的集号；解析不出缺省（列表排最后） */
  episode?: number;
  season: number;
  isSpecial: boolean;
  hasNfo: boolean;
  sizeBytes?: number;
  title?: string;
  aired?: string;
  runtimeMinutes?: number;
}

export interface SeasonInfo {
  season: number;
  episodes: EpisodeFile[];
}

export interface ShowDetail {
  item: LibraryItem;
  seasons: SeasonInfo[];
}

/* ---------- 刮削 ---------- */

export type ScrapeSource = "tmdb" | "bangumi" | "douban";

export interface ScrapePreview {
  metadata: MediaMetadata;
  episodes: EpisodeMetadata[];
}

export interface ScrapeResult {
  nfoPath: string;
  posterPath?: string;
  episodeNfos: number;
}

/* ---------- 整理（plan → 用户确认 → apply） ---------- */

export type PlanStatus = "move" | "skip" | "skip_already_in_place";

export interface PlanItem {
  from: string;
  to?: string;
  status: PlanStatus;
  reason?: string;
}

export interface OrganizePlan {
  showRoot: string;
  nfoPath: string;
  items: PlanItem[];
  toMove: number;
  skipped: number;
  alreadyInPlace: number;
}

/** preload 注入的 window.media 形状 */
export interface MediaApi {
  pickLibrary(): Promise<string | null>;
  scanLibrary(root: string): Promise<LibraryItem[]>;
  showDetail(dir: string): Promise<ShowDetail | null>;
  imageDataUrl(path: string): Promise<string | null>;
  reveal(path: string): void;

  searchMetadata(
    source: ScrapeSource,
    keyword: string,
    mediaType: MediaKind,
  ): Promise<SearchResult[]>;
  scrapePreview(
    source: ScrapeSource,
    id: string,
    mediaType: MediaKind,
  ): Promise<ScrapePreview>;
  scrapeApply(
    dir: string,
    source: ScrapeSource,
    id: string,
    mediaType: MediaKind,
  ): Promise<ScrapeResult>;

  readNfo(dir: string): Promise<MediaMetadata | null>;
  writeNfo(dir: string, metadata: MediaMetadata): Promise<string>;

  planOrganize(root: string, dir: string): Promise<OrganizePlan>;
  applyOrganize(root: string, dir: string): Promise<PlanItem[]>;
  /** 返回清除的缓存条数 */
  clearCache(): Promise<number>;
}

/** preload 注入的宿主状态桥（独立应用没有宿主，initial 回退 dark） */
export interface HostStateBridge {
  readonly initial: HostState;
  onChanged(cb: (state: HostState) => void): void;
}

declare global {
  interface Window {
    media?: MediaApi;
    hostState?: HostStateBridge;
  }
}
