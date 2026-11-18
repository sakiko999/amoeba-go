/**
 * 豆瓣 provider（按旧 core 规格重写）：只支持按 ID 刮削 —— 豆瓣没有稳定的搜索
 * API（suggest 端点结构不稳定），旧项目决策如此，搜索里不暴露豆瓣。
 *
 * 必须知道的点：
 * - 必须带 Referer: https://movie.douban.com/，否则 invalid_request
 * - 剧集请求 movie 路径后 **301 到 /v2/tv/{id}**，靠 net.fetch 默认跟随重定向
 * - 请求频率必须低（连续请求会被限流返回空响应）——有缓存兜底
 */

import {
  type Actor,
  type MediaMetadata,
  type MetadataProvider,
  newMetadata,
  getJsonCached,
} from "@tabs/platform-kit/provider";

const API = "https://m.douban.com/rexxar/api/v2";

interface DoubanPerson {
  name?: string;
}

interface DoubanSubject {
  id?: string;
  title?: string;
  original_title?: string;
  intro?: string;
  year?: string;
  release_date?: string;
  rating?: { value?: number; count?: number };
  genres?: string[];
  cover_url?: string;
  pic?: { large?: string };
  actors?: DoubanPerson[];
  directors?: DoubanPerson[];
  episodes_count?: number;
  is_tv?: boolean;
}

export function mapSubject(raw: unknown, requestedId: string): MediaMetadata {
  const s = raw as DoubanSubject;
  const title = (s.title ?? "").trim();
  if (title === "") throw new Error(`[douban] 条目 ${requestedId} 缺少标题`);

  const isTv = s.is_tv === true || (s.episodes_count ?? 0) > 0;
  const m = newMetadata(isTv ? "tv" : "movie", title);
  const originalTitle = (s.original_title ?? "").trim();
  if (originalTitle !== "" && originalTitle !== title)
    m.originalTitle = originalTitle;
  const plot = (s.intro ?? "").trim();
  if (plot !== "") m.plot = plot;

  const date = (s.release_date ?? "").trim();
  if (/^\d{4}-\d{2}-\d{2}$/.test(date)) m.premiered = date;

  m.uniqueIds.push({ source: "douban", value: String(s.id ?? requestedId) });
  m.genres = (s.genres ?? []).map((g) => g.trim()).filter(Boolean);

  const score = s.rating?.value;
  const count = s.rating?.count;
  if (score !== undefined && score > 0) {
    m.ratings.push({
      source: "douban",
      value: score,
      votes: count !== undefined && count > 0 ? count : undefined,
    });
  }

  const poster = (s.cover_url ?? s.pic?.large ?? "").trim();
  if (poster !== "") m.posterUrl = poster;

  const cast: Actor[] = [
    ...(s.directors ?? []).map((p, i) => ({
      name: (p.name ?? "").trim(),
      role: "导演",
      order: i,
    })),
    ...(s.actors ?? []).map((p, i) => ({
      name: (p.name ?? "").trim(),
      order: i + (s.directors?.length ?? 0),
    })),
  ].filter((a) => a.name !== "" && a.name !== "undefined");
  m.actors = cast;

  if (isTv && (s.episodes_count ?? 0) > 0) {
    m.episodeCount = s.episodes_count;
    m.seasonCount = 1;
  }
  return m;
}

export async function fetchDouban(id: string): Promise<MediaMetadata> {
  const raw = await getJsonCached(
    "douban",
    ["douban", "subject", id],
    `${API}/movie/${id}`,
    {
      Referer: "https://movie.douban.com/",
    },
  );
  return mapSubject(raw, id);
}

/** 组件入口用工厂：豆瓣没有稳定的搜索 API，只能按 ID 刮削；无每集接口。 */
export function createDoubanProvider(): MetadataProvider {
  return {
    source: "douban",
    search: () => {
      throw new Error("[douban] 豆瓣没有稳定的搜索 API，只能按 ID 刮削");
    },
    fetchMetadata: (id) => fetchDouban(id),
    fetchEpisodes: async () => [],
  };
}