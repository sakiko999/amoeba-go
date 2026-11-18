/**
 * 生成 Kodi 规范 NFO XML（移植自旧 core 的 nfo/writer.rs）。
 * 写错会让 Kodi 静默不识别的约束：
 * - <uniqueid> Required，只有一个 default="true"，type 不能取值 "default"
 * - <ratings> 嵌套结构（<rating name max default><value><votes>），name 不能用 "default"
 * - 剧集 <tvshow> / 电影 <movie>；episodeguide v19+ 是 JSON
 * - 空值不写标签（空标签会覆盖 Kodi 已刮削的内容）
 */

import type {
  EpisodeMetadata,
  MediaMetadata,
  Rating,
  UniqueId,
} from "../shared/model";

function esc(s: string): string {
  // CRLF 规范化为 LF，避免 &#13; 噪声
  const normalized = s.includes("\r")
    ? s.replaceAll("\r\n", "\n").replaceAll("\r", "\n")
    : s;
  return normalized
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function escAttr(s: string): string {
  return esc(s).replaceAll("'", "&apos;");
}

/** 评分保留 1 位小数，整数不带 .0 */
function formatRating(v: number): string {
  return Number.isInteger(v) ? String(v) : v.toFixed(1);
}

class Xml {
  private out = '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n';
  private depth = 0;

  open(tag: string, attrs: Record<string, string> = {}): this {
    const a = Object.entries(attrs)
      .map(([k, v]) => ` ${k}="${escAttr(v)}"`)
      .join("");
    this.out += `${" ".repeat(this.depth * 4)}<${tag}${a}>\n`;
    this.depth += 1;
    return this;
  }

  close(tag: string): this {
    this.depth -= 1;
    this.out += `${" ".repeat(this.depth * 4)}</${tag}>\n`;
    return this;
  }

  /** 有值才写；空字符串视为缺失 */
  elem(tag: string, content: string | undefined): this {
    if (content === undefined || content === "") return this;
    return this.raw(`<${tag}>${esc(content)}</${tag}>`);
  }

  raw(fragment: string): this {
    this.out += `${" ".repeat(this.depth * 4)}${fragment}\n`;
    return this;
  }

  toString(): string {
    return this.out;
  }
}

function writeUniqueIds(x: Xml, ids: UniqueId[]): void {
  ids.forEach((uid, i) => {
    x.raw(
      `<uniqueid type="${escAttr(uid.source)}" default="${i === 0}">${esc(uid.value)}</uniqueid>`,
    );
  });
}

function writeRatings(x: Xml, ratings: Rating[]): void {
  if (ratings.length === 0) return;
  x.open("ratings");
  ratings.forEach((r, i) => {
    x.open("rating", { name: r.source, max: "10", default: String(i === 0) });
    x.elem("value", formatRating(r.value));
    x.elem("votes", r.votes !== undefined ? String(r.votes) : undefined);
    x.close("rating");
  });
  x.close("ratings");
}

function writeActors(x: Xml, actors: MediaMetadata["actors"]): void {
  for (const a of actors) {
    x.open("actor");
    x.elem("name", a.name);
    x.elem("role", a.role);
    x.elem("order", String(a.order));
    x.elem("thumb", a.thumbUrl);
    x.close("actor");
  }
}

/** v19+ 的 JSON 格式 episodeguide：{"tmdb":"1399","imdb":"tt0944947"} */
function episodeGuideJson(meta: MediaMetadata): string | undefined {
  if (meta.uniqueIds.length === 0) return undefined;
  const pairs = meta.uniqueIds.map((u) => `"${u.source}": "${u.value}"`);
  return `{${pairs.join(", ")}}`;
}

export function writeShowNfo(meta: MediaMetadata): string {
  const x = new Xml();
  const guide = episodeGuideJson(meta);
  x.open("tvshow", guide !== undefined ? { episodeguide: guide } : {});
  x.elem("title", meta.title);
  x.elem("originaltitle", meta.originalTitle);
  x.elem("plot", meta.plot);
  writeUniqueIds(x, meta.uniqueIds);
  for (const g of meta.genres) x.elem("genre", g);
  x.elem("premiered", meta.premiered);
  x.elem("status", meta.status);
  for (const s of meta.studios) x.elem("studio", s);
  writeRatings(x, meta.ratings);
  writeActors(x, meta.actors);
  x.elem(
    "season",
    meta.seasonCount !== undefined ? String(meta.seasonCount) : undefined,
  );
  x.elem(
    "episode",
    meta.episodeCount !== undefined ? String(meta.episodeCount) : undefined,
  );
  x.close("tvshow");
  return x.toString();
}

export function writeMovieNfo(meta: MediaMetadata): string {
  const x = new Xml();
  x.open("movie");
  x.elem("title", meta.title);
  x.elem("originaltitle", meta.originalTitle);
  x.elem("plot", meta.plot);
  writeUniqueIds(x, meta.uniqueIds);
  for (const g of meta.genres) x.elem("genre", g);
  x.elem("premiered", meta.premiered);
  for (const s of meta.studios) x.elem("studio", s);
  writeRatings(x, meta.ratings);
  writeActors(x, meta.actors);
  x.close("movie");
  return x.toString();
}

export function writeMetadataNfo(meta: MediaMetadata): string {
  return meta.mediaType === "tv" ? writeShowNfo(meta) : writeMovieNfo(meta);
}

/** 单集 NFO：<episodedetails>。播出日期是 <aired>（不是剧集的 <premiered>） */
export function writeEpisodeNfo(ep: EpisodeMetadata): string {
  const x = new Xml();
  x.open("episodedetails");
  x.elem("title", ep.title);
  x.elem("originaltitle", ep.originalTitle);
  x.elem("plot", ep.plot);
  x.elem("season", String(ep.season));
  x.elem("episode", String(ep.episode));
  x.elem("aired", ep.aired);
  x.elem("thumb", ep.thumbUrl);
  x.elem(
    "runtime",
    ep.runtimeMinutes !== undefined ? String(ep.runtimeMinutes) : undefined,
  );
  writeUniqueIds(x, ep.uniqueIds);
  if (ep.rating !== undefined) writeRatings(x, [ep.rating]);
  writeActors(x, ep.actors);
  x.close("episodedetails");
  return x.toString();
}
