/**
 * 平台统一的媒体数据模型：provider 输出、域服务（刮削/NFO）与页面共用同一份定义。
 * 各 provider 把上游响应映射成本结构；NFO writer/parser 只认本结构，不知道数据来源。
 */

export type MediaType = "tv" | "movie";

export interface UniqueId {
  /** 如 "tmdb" / "bangumi"；不能取值 "default"（Kodi 禁止） */
  source: string;
  value: string;
}

export interface Rating {
  source: string;
  /** 0-10 分制 */
  value: number;
  votes?: number;
}

export interface Actor {
  name: string;
  role?: string;
  /** 展示顺序，0 为第一位（Kodi 用 <order> 排序） */
  order: number;
  thumbUrl?: string;
}

/** 搜索命中：刻意比 MediaMetadata 轻 —— 只够用户从候选里挑一个 */
export interface SearchResult {
  source: string;
  id: string;
  title: string;
  originalTitle?: string;
  year?: string;
  mediaType: MediaType;
  posterUrl?: string;
  rating?: number;
  ratingVotes?: number;
  plot?: string;
  /** 集数，用于区分同作品的不同季度/篇章 */
  episodeCount?: number;
}

/** 单集元数据（<episodedetails>）。播出日期是 <aired>（剧集级是 <premiered>，不可混） */
export interface EpisodeMetadata {
  season: number;
  episode: number;
  title: string;
  originalTitle?: string;
  plot?: string;
  aired?: string;
  /** 分钟 */
  runtimeMinutes?: number;
  rating?: Rating;
  actors: Actor[];
  uniqueIds: UniqueId[];
  /** 每集剧照 URL（NFO 顶层 <thumb>） */
  thumbUrl?: string;
}

/** 归一化的作品元数据 —— 只含 NFO writer 真正会输出的字段 */
export interface MediaMetadata {
  mediaType: MediaType;
  title: string;
  originalTitle?: string;
  plot?: string;
  /** yyyy-mm-dd */
  premiered?: string;
  genres: string[];
  studios: string[];
  status?: string;
  ratings: Rating[];
  actors: Actor[];
  uniqueIds: UniqueId[];
  posterUrl?: string;
  seasonCount?: number;
  episodeCount?: number;
}

export function newMetadata(
  mediaType: MediaType,
  title: string,
): MediaMetadata {
  return {
    mediaType,
    title,
    genres: [],
    studios: [],
    ratings: [],
    actors: [],
    uniqueIds: [],
  };
}

/** 从 premiered 切年份（前 4 位全数字才有效） */
export function metadataYear(meta: MediaMetadata): string | undefined {
  const y = meta.premiered?.slice(0, 4);
  return y !== undefined && /^\d{4}$/.test(y) ? y : undefined;
}
