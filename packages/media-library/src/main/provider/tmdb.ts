/**
 * TMDB provider（按旧 core 规格重写：v4 read access token 打 v3 端点）。
 * - 必须加 append_to_response=credits,external_ids，否则没有演员/IMDB ID
 * - 每集按季逐个请求（N 季 = N 次），这是 TMDB 相对 Bangumi 的成本
 * - 中文标题靠 language=zh-CN
 * - 凭证只从环境变量 TMDB_TOKEN 读，不经前端传递
 */

import type {
  Actor,
  EpisodeMetadata,
  MediaMetadata,
  MediaType,
  SearchResult,
} from "../../shared/model";
import { newMetadata } from "../../shared/model";
import { ProviderError, getJsonCached } from "./http";

const API = "https://api.themoviedb.org/3";
const IMG = "https://image.tmdb.org/t/p";
const LANG = "zh-CN";

interface TmdbPerson {
  name?: string;
  character?: string;
  order?: number;
  profile_path?: string | null;
}

interface TmdbSeason {
  season_number?: number;
  episode_count?: number;
}

interface TmdbTv {
  id?: number;
  name?: string;
  original_name?: string;
  overview?: string;
  first_air_date?: string;
  genres?: { name?: string }[];
  production_companies?: { name?: string }[];
  status?: string;
  vote_average?: number;
  vote_count?: number;
  poster_path?: string | null;
  number_of_seasons?: number;
  number_of_episodes?: number;
  seasons?: TmdbSeason[];
  credits?: { cast?: TmdbPerson[] };
  external_ids?: { imdb_id?: string | null };
}

interface TmdbMovie {
  id?: number;
  title?: string;
  original_title?: string;
  overview?: string;
  release_date?: string;
  genres?: { name?: string }[];
  production_companies?: { name?: string }[];
  status?: string;
  vote_average?: number;
  vote_count?: number;
  poster_path?: string | null;
  runtime?: number;
  credits?: { cast?: TmdbPerson[] };
  external_ids?: { imdb_id?: string | null };
}

interface TmdbSeasonDetail {
  episodes?: {
    episode_number?: number;
    name?: string;
    overview?: string;
    air_date?: string | null;
    runtime?: number | null;
    still_path?: string | null;
    vote_average?: number;
  }[];
}

function authHeaders(): Record<string, string> {
  const token = process.env.TMDB_TOKEN;
  if (token === undefined || token.trim() === "") {
    throw new ProviderError(
      "tmdb",
      "未设置 TMDB_TOKEN 环境变量（v4 Read Access Token），无法访问 TMDB。",
    );
  }
  return { Authorization: `Bearer ${token}` };
}

async function get(path: string, cacheParts: string[]): Promise<unknown> {
  return getJsonCached("tmdb", cacheParts, `${API}${path}`, authHeaders());
}

const posterUrl = (p: string | null | undefined): string | undefined =>
  p != null && p !== "" ? `${IMG}/w500${p}` : undefined;
const thumbUrl = (p: string | null | undefined): string | undefined =>
  p != null && p !== "" ? `${IMG}/w300${p}` : undefined;

function mapCast(cast: TmdbPerson[] | undefined): Actor[] {
  return (cast ?? []).slice(0, 15).map((c, i) => ({
    name: (c.name ?? "").trim(),
    role: (c.character ?? "").trim() || undefined,
    order: c.order ?? i,
    thumbUrl:
      c.profile_path != null ? `${IMG}/w185${c.profile_path}` : undefined,
  }));
}

export function mapTvDetail(raw: unknown, id: string): MediaMetadata {
  const tv = raw as TmdbTv;
  const title = (tv.name ?? "").trim() || (tv.original_name ?? "").trim();
  if (title === "") throw new ProviderError("tmdb", `条目 ${id} 缺少标题`);
  const m = newMetadata("tv", title);
  m.originalTitle =
    (tv.original_name ?? "").trim() !== title
      ? tv.original_name?.trim()
      : undefined;
  m.plot = (tv.overview ?? "").trim() || undefined;
  m.premiered = (tv.first_air_date ?? "").trim() || undefined;
  m.status = (tv.status ?? "").trim() || undefined;
  m.genres = (tv.genres ?? []).map((g) => g.name ?? "").filter(Boolean);
  m.studios = (tv.production_companies ?? [])
    .map((c) => c.name ?? "")
    .filter(Boolean);
  m.uniqueIds.push({ source: "tmdb", value: String(tv.id ?? id) });
  const imdb = tv.external_ids?.imdb_id;
  if (imdb != null && imdb !== "")
    m.uniqueIds.push({ source: "imdb", value: imdb });
  if ((tv.vote_average ?? 0) > 0) {
    m.ratings.push({
      source: "themoviedb",
      value: tv.vote_average ?? 0,
      votes: (tv.vote_count ?? 0) > 0 ? tv.vote_count : undefined,
    });
  }
  m.actors = mapCast(tv.credits?.cast);
  m.posterUrl = posterUrl(tv.poster_path);
  m.seasonCount = tv.number_of_seasons;
  m.episodeCount = tv.number_of_episodes;
  return m;
}

export function mapMovieDetail(raw: unknown, id: string): MediaMetadata {
  const mv = raw as TmdbMovie;
  const title = (mv.title ?? "").trim() || (mv.original_title ?? "").trim();
  if (title === "") throw new ProviderError("tmdb", `条目 ${id} 缺少标题`);
  const m = newMetadata("movie", title);
  m.originalTitle =
    (mv.original_title ?? "").trim() !== title
      ? mv.original_title?.trim()
      : undefined;
  m.plot = (mv.overview ?? "").trim() || undefined;
  m.premiered = (mv.release_date ?? "").trim() || undefined;
  m.status = (mv.status ?? "").trim() || undefined;
  m.genres = (mv.genres ?? []).map((g) => g.name ?? "").filter(Boolean);
  m.studios = (mv.production_companies ?? [])
    .map((c) => c.name ?? "")
    .filter(Boolean);
  m.uniqueIds.push({ source: "tmdb", value: String(mv.id ?? id) });
  const imdb = mv.external_ids?.imdb_id;
  if (imdb != null && imdb !== "")
    m.uniqueIds.push({ source: "imdb", value: imdb });
  if ((mv.vote_average ?? 0) > 0) {
    m.ratings.push({
      source: "themoviedb",
      value: mv.vote_average ?? 0,
      votes: (mv.vote_count ?? 0) > 0 ? mv.vote_count : undefined,
    });
  }
  m.actors = mapCast(mv.credits?.cast);
  m.posterUrl = posterUrl(mv.poster_path);
  return m;
}

export function mapTvSearch(raw: unknown, limit: number): SearchResult[] {
  const results = ((raw as { results?: TmdbTv[] }).results ?? []).slice(
    0,
    limit,
  );
  return results.map((tv) => ({
    source: "tmdb",
    id: String(tv.id ?? ""),
    title: (tv.name ?? tv.original_name ?? "").trim(),
    originalTitle:
      (tv.original_name ?? "").trim() !== (tv.name ?? "").trim()
        ? tv.original_name?.trim()
        : undefined,
    year: (tv.first_air_date ?? "").slice(0, 4) || undefined,
    mediaType: "tv",
    posterUrl: posterUrl(tv.poster_path),
    rating: (tv.vote_average ?? 0) > 0 ? tv.vote_average : undefined,
    ratingVotes: (tv.vote_count ?? 0) > 0 ? tv.vote_count : undefined,
    plot: (tv.overview ?? "").trim() || undefined,
  }));
}

export function mapMovieSearch(raw: unknown, limit: number): SearchResult[] {
  const results = ((raw as { results?: TmdbMovie[] }).results ?? []).slice(
    0,
    limit,
  );
  return results.map((mv) => ({
    source: "tmdb",
    id: String(mv.id ?? ""),
    title: (mv.title ?? mv.original_title ?? "").trim(),
    originalTitle:
      (mv.original_title ?? "").trim() !== (mv.title ?? "").trim()
        ? mv.original_title?.trim()
        : undefined,
    year: (mv.release_date ?? "").slice(0, 4) || undefined,
    mediaType: "movie",
    posterUrl: posterUrl(mv.poster_path),
    rating: (mv.vote_average ?? 0) > 0 ? mv.vote_average : undefined,
    ratingVotes: (mv.vote_count ?? 0) > 0 ? mv.vote_count : undefined,
    plot: (mv.overview ?? "").trim() || undefined,
  }));
}

export function mapSeasonEpisodes(
  raw: unknown,
  season: number,
): EpisodeMetadata[] {
  const data = raw as TmdbSeasonDetail;
  return (data.episodes ?? [])
    .map((e) => ({
      season,
      episode: e.episode_number ?? 0,
      title: (e.name ?? "").trim() || `第 ${e.episode_number ?? "?"} 集`,
      plot: (e.overview ?? "").trim() || undefined,
      aired: (e.air_date ?? "")?.trim() || undefined,
      runtimeMinutes:
        e.runtime != null && e.runtime > 0 ? e.runtime : undefined,
      rating: undefined,
      actors: [],
      uniqueIds: [],
      thumbUrl: thumbUrl(e.still_path),
    }))
    .filter((e) => e.episode > 0)
    .sort((a, b) => a.episode - b.episode);
}

// —— 网络层

export async function fetchTmdb(
  id: string,
  mediaType: MediaType,
): Promise<MediaMetadata> {
  const path = mediaType === "tv" ? `/tv/${id}` : `/movie/${id}`;
  const raw = await get(
    `${path}?language=${LANG}&append_to_response=credits,external_ids`,
    ["tmdb", mediaType, id],
  );
  return mediaType === "tv" ? mapTvDetail(raw, id) : mapMovieDetail(raw, id);
}

export async function searchTmdb(
  keyword: string,
  mediaType: MediaType,
  limit = 20,
): Promise<SearchResult[]> {
  if (keyword.trim() === "") return [];
  const path = mediaType === "tv" ? "/search/tv" : "/search/movie";
  const raw = await get(
    `${path}?language=${LANG}&query=${encodeURIComponent(keyword)}`,
    ["tmdb", "search", mediaType, keyword],
  );
  return mediaType === "tv"
    ? mapTvSearch(raw, limit)
    : mapMovieSearch(raw, limit);
}

/** 按季逐集抓取（N 季 = N 次请求）。任一季失败只警告并跳过 */
export async function fetchTmdbEpisodes(
  id: string,
  seasonCount: number,
): Promise<EpisodeMetadata[]> {
  const all: EpisodeMetadata[] = [];
  for (let s = 1; s <= seasonCount; s++) {
    try {
      const raw = await get(`/tv/${id}/season/${s}?language=${LANG}`, [
        "tmdb",
        "season",
        id,
        String(s),
      ]);
      all.push(...mapSeasonEpisodes(raw, s));
    } catch (err) {
      console.warn(`tmdb season ${s} fetch failed for ${id}:`, err);
    }
  }
  return all;
}
