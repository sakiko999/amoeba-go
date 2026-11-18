/**
 * Bangumi（bgm.tv）provider（移植自旧 core 的 provider/bangumi.rs，测试见 bangumi.test.ts）。
 *
 * - 读接口免 token，但必须带 User-Agent（默认 UA 会被拒）；BGM_TOKEN 可选
 * - `type` 是 int 枚举：1 书籍 / 2 动画 / 3 音乐 / 4 游戏 / 6 三次元 ——
 *   书籍/音乐直接报错（给 OST 生成 tvshow.nfo 是垃圾输出）
 * - tags 是用户自由标注（含「妹控」这类），不用作 genre
 * - 中文名陷阱：不少条目 name 与 name_cn 完全相同，真中文名在 infobox「别名」里；
 *   搜索与抓取必须用同一套取名规则，否则搜到的名字和写进 NFO 的对不上
 * - 每集一次请求拿全集（TMDB 要按季逐个请求）
 */

import {
  type EpisodeMetadata,
  type MediaMetadata,
  type MediaType,
  type MetadataProvider,
  type SearchResult,
  newMetadata,
  NotFoundError,
  ProviderError,
  getJsonCached,
  postJsonCached,
} from "@tabs/platform-kit/provider";

const API_BASE = "https://api.bgm.tv/v0";

const SUBJECT_TYPE = { BOOK: 1, ANIME: 2, MUSIC: 3, GAME: 4, REAL: 6 } as const;

interface RawSubject {
  id?: number;
  type?: number;
  name?: string;
  name_cn?: string;
  summary?: string;
  date?: string;
  total_episodes?: number;
  eps?: number;
  images?: { large?: string };
  rating?: { score?: number; total?: number };
  infobox?: RawInfoboxItem[];
}

interface RawInfoboxItem {
  key: string;
  value: unknown;
}

interface RawEpisode {
  id?: number;
  ep?: number;
  sort?: number;
  name?: string;
  name_cn?: string;
  desc?: string;
  airdate?: string;
  duration_seconds?: number;
}

/** infobox 指定 key 的所有取值（字符串或 [{v:"..."}] 数组两种形态） */
function infoboxValues(
  items: RawInfoboxItem[] | undefined,
  keys: string[],
): string[] {
  const out: string[] = [];
  for (const item of items ?? []) {
    if (!keys.includes(item.key.trim())) continue;
    if (typeof item.value === "string") {
      const t = item.value.trim();
      if (t !== "") out.push(t);
    } else if (Array.isArray(item.value)) {
      for (const v of item.value) {
        if (
          v !== null &&
          typeof v === "object" &&
          typeof (v as { v?: unknown }).v === "string"
        ) {
          const t = (v as { v: string }).v.trim();
          if (t !== "") out.push(t);
        }
      }
    }
  }
  return out;
}

function infoboxFirstAlias(
  items: RawInfoboxItem[] | undefined,
): string | undefined {
  return infoboxValues(items, ["别名", "別名", "英文名"])[0];
}

/** 宽松校验 yyyy-mm-dd（date 偶尔只有年份或空串） */
function isIsoDate(s: string): boolean {
  return s.length === 10 && s[4] === "-" && s[7] === "-";
}

/** 搜索与抓取共用的取名规则：有效中文名 = 与原名不同的 name_cn，否则别名，否则原名 */
function pickTitle(s: RawSubject): { title: string; originalTitle?: string } {
  const nameCn = (s.name_cn ?? "").trim();
  const name = (s.name ?? "").trim();
  const alias = infoboxFirstAlias(s.infobox)?.trim();

  const cn =
    nameCn !== "" && nameCn !== name
      ? nameCn
      : alias !== undefined && alias !== "" && alias !== name
        ? alias
        : undefined;
  const title = cn ?? name;
  return {
    title,
    originalTitle: name !== "" && name !== title ? name : undefined,
  };
}

export function mapSubject(
  raw: unknown,
  requestedId: string,
  mediaType: MediaType,
): MediaMetadata {
  const s = raw as RawSubject;
  const type = s.type ?? 0;
  if (type === SUBJECT_TYPE.BOOK || type === SUBJECT_TYPE.MUSIC) {
    const kind = type === SUBJECT_TYPE.BOOK ? "书籍" : "音乐";
    throw new ProviderError(
      "bangumi",
      `条目 ${requestedId} 是${kind}（type=${type}），不是影视作品，无法生成 NFO。请确认 ID 是否正确。`,
    );
  }
  if (
    ![SUBJECT_TYPE.ANIME, SUBJECT_TYPE.GAME, SUBJECT_TYPE.REAL, 0].includes(
      type,
    )
  ) {
    throw new ProviderError(
      "bangumi",
      `条目 ${requestedId} 的类型 ${type} 暂不支持`,
    );
  }

  const { title, originalTitle } = pickTitle(s);
  if (title === "") {
    throw new ProviderError("bangumi", `条目 ${requestedId} 缺少标题字段`);
  }

  const m = newMetadata(mediaType, title);
  m.originalTitle = originalTitle;
  const plot = (s.summary ?? "").trim();
  if (plot !== "") m.plot = plot;
  const date = (s.date ?? "").trim();
  if (isIsoDate(date)) m.premiered = date;
  m.uniqueIds.push({ source: "bangumi", value: String(s.id ?? requestedId) });
  const poster = s.images?.large?.trim();
  if (poster !== undefined && poster !== "") m.posterUrl = poster;

  // 只取「动画制作」；「製作」是制作委员会长串，是垃圾数据
  m.studios = infoboxValues(s.infobox, ["动画制作", "動畫製作", "制作公司"]);
  if ((s.rating?.score ?? 0) > 0) {
    m.ratings.push({
      source: "bangumi",
      value: s.rating?.score ?? 0,
      votes: (s.rating?.total ?? 0) > 0 ? s.rating?.total : undefined,
    });
  }
  const eps = s.total_episodes ?? s.eps;
  if (eps !== undefined && eps > 0) {
    m.episodeCount = eps;
    // Bangumi 条目通常对应单季（多季是独立条目）
    m.seasonCount = 1;
  }
  return m;
}

export function mapSearch(raw: unknown, limit: number): SearchResult[] {
  const data = (raw as { data?: RawSubject[] }).data ?? [];
  const out: SearchResult[] = [];
  for (const s of data) {
    const { title, originalTitle } = pickTitle(s);
    if (title === "") continue;
    const date = (s.date ?? "").trim();
    const y = date.slice(0, 4);
    out.push({
      source: "bangumi",
      id: String(s.id ?? ""),
      title,
      originalTitle,
      year: /^\d{4}$/.test(y) ? y : undefined,
      // 搜索端点不区分 tv/movie，动漫占绝大多数，统一按 Tv；电影用户可在前端改
      mediaType: "tv",
      posterUrl: nonEmpty(s.images?.large?.trim()),
      rating: (s.rating?.score ?? 0) > 0 ? s.rating?.score : undefined,
      ratingVotes: (s.rating?.total ?? 0) > 0 ? s.rating?.total : undefined,
      plot: nonEmpty((s.summary ?? "").trim()),
      episodeCount: positive(s.total_episodes ?? s.eps),
    });
    if (out.length >= limit) break;
  }
  return out;
}

export function mapEpisodes(
  raw: unknown,
  subjectId: string,
): EpisodeMetadata[] {
  const data = (raw as { data?: RawEpisode[] }).data ?? [];
  const out: EpisodeMetadata[] = [];
  for (const e of data) {
    const num = e.ep ?? e.sort;
    // 小数集号（5.5 特别篇）与 0 跳过 —— NFO 的 episode 只接受整数
    if (num === undefined || num <= 0 || !Number.isInteger(num)) continue;

    const nameCn = (e.name_cn ?? "").trim();
    const name = (e.name ?? "").trim();
    const title = nameCn !== "" ? nameCn : name !== "" ? name : `第 ${num} 集`;

    const ep: EpisodeMetadata = {
      season: 1,
      episode: num,
      title,
      originalTitle: name !== "" && name !== title ? name : undefined,
      plot: nonEmpty((e.desc ?? "").trim()),
      aired: isIsoDate((e.airdate ?? "").trim())
        ? (e.airdate ?? "").trim()
        : undefined,
      // 秒 → 分钟向上取整（90 秒的短片应是 2 分钟而非 0）
      runtimeMinutes:
        (e.duration_seconds ?? 0) > 0
          ? Math.ceil((e.duration_seconds ?? 0) / 60)
          : undefined,
      rating: undefined,
      actors: [],
      uniqueIds: [{ source: "bangumi", value: subjectId }],
      thumbUrl: undefined,
    };
    if ((e.id ?? 0) > 0) {
      ep.uniqueIds.push({ source: "bangumi-episode", value: String(e.id) });
    }
    out.push(ep);
  }
  out.sort((a, b) => a.episode - b.episode);
  return out;
}

function nonEmpty(s: string | undefined): string | undefined {
  return s !== undefined && s !== "" ? s : undefined;
}

function positive(n: number | undefined): number | undefined {
  return n !== undefined && n > 0 ? n : undefined;
}

// —— 网络层

export async function fetchBangumi(
  id: string,
  mediaType: MediaType,
): Promise<MediaMetadata> {
  const raw = await getJsonCached(
    "bangumi",
    ["bangumi", "subject", id],
    `${API_BASE}/subjects/${id}`,
  );
  return mapSubject(raw, id, mediaType);
}

export async function searchBangumi(
  keyword: string,
  limit = 20,
): Promise<SearchResult[]> {
  if (keyword.trim() === "") return [];
  // POST + JSON body（GET 会 404）；filter.type 只放动画与三次元 —— 书籍/音乐无法生成 NFO
  const raw = await postJsonCached(
    "bangumi",
    ["bangumi", "search", keyword, String(limit)],
    `${API_BASE}/search/subjects?limit=${limit}`,
    { keyword, filter: { type: [SUBJECT_TYPE.ANIME, SUBJECT_TYPE.REAL] } },
  );
  return mapSearch(raw, limit);
}

/** 每集（type=0 本篇，不含 OP/ED/SP 条目）。条目不存在时返回空集而非失败 */
export async function fetchBangumiEpisodes(
  id: string,
): Promise<EpisodeMetadata[]> {
  try {
    const raw = await getJsonCached(
      "bangumi",
      ["bangumi", "episodes", id],
      `${API_BASE}/episodes?subject_id=${id}&type=0`,
    );
    return mapEpisodes(raw, id);
  } catch (err) {
    if (err instanceof NotFoundError) return [];
    throw err;
  }
}

/** 组件入口用工厂：搜索结果与详情共用同一套映射（见上方 map* 纯函数）。 */
export function createBangumiProvider(): MetadataProvider {
  return {
    source: "bangumi",
    search: (keyword) => searchBangumi(keyword),
    fetchMetadata: (id, mediaType) => fetchBangumi(id, mediaType),
    fetchEpisodes: (_meta, id) => fetchBangumiEpisodes(id),
  };
}