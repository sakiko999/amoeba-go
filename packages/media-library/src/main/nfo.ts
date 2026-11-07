/**
 * NFO 解析。
 * - parseShowNfo / parseEpisodeNfo：扫描列表用的轻量字段抽取
 * - parseMetadataNfo：完整作品级解析（编辑回读 / 整理时复用），与 nfoWriter 的
 *   <ratings> 嵌套、<uniqueid>、<actor> 结构对应 —— 往返写入不丢字段。
 * 标签约定：作品级 <premiered>、单集 <aired>，不可混；空标签视为缺失。
 */

import type {
  Actor,
  MediaMetadata,
  MediaType,
  Rating,
  UniqueId,
} from "../shared/model";
import { newMetadata } from "../shared/model";

export interface ShowNfo {
  title: string;
  premiered?: string;
  plot?: string;
  rating?: number;
  genres: string[];
  seasonCount?: number;
  episodeCount?: number;
}

export interface EpisodeNfo {
  title?: string;
  aired?: string;
  runtimeMinutes?: number;
}

/** 先在入口统一换行，避免 plot 中的 CRLF 原样进入 UI / 回写 */
const normalize = (s: string): string =>
  s.replaceAll("\r\n", "\n").replaceAll("\r", "\n");

const decodeEntities = (s: string): string =>
  s.replace(
    /&(amp|lt|gt|quot|apos|#(\d+)|#x([0-9a-fA-F]+));/g,
    (_all, name: string, dec?: string, hex?: string) => {
      switch (name) {
        case "amp":
          return "&";
        case "lt":
          return "<";
        case "gt":
          return ">";
        case "quot":
          return '"';
        case "apos":
          return "'";
        default:
          return String.fromCodePoint(
            Number(
              hex !== undefined ? Number.parseInt(hex, 16) : Number(dec ?? "0"),
            ),
          );
      }
    },
  );

const tagPattern = (name: string): RegExp =>
  new RegExp(`<${name}(?:\\s[^>]*)?>([^<]*)</${name}>`, "i");

/** 取第一个 <name>…</name> 的文本（无嵌套假设） */
const tag = (xml: string, name: string): string | undefined => {
  const m = xml.match(tagPattern(name));
  return m ? decodeEntities(m[1] ?? "").trim() || undefined : undefined;
};

/** 取所有同名标签的文本（genre/studio 可重复） */
const allTags = (xml: string, name: string): string[] => {
  const re = new RegExp(tagPattern(name).source, "gi");
  return [...xml.matchAll(re)]
    .map((m) => decodeEntities(m[1] ?? "").trim())
    .filter(Boolean);
};

interface Block {
  attrs: string;
  inner: string;
}

/** 取所有 <name …>…</name> 块（容忍嵌套，非贪婪） */
const blocks = (xml: string, name: string): Block[] => {
  const re = new RegExp(`<${name}(\\s[^>]*)?>([\\s\\S]*?)</${name}>`, "gi");
  return [...xml.matchAll(re)].map((m) => ({
    attrs: m[1] ?? "",
    inner: m[2] ?? "",
  }));
};

const attr = (attrs: string, name: string): string | undefined => {
  const m = attrs.match(
    new RegExp(`\\b${name}\\s*=\\s*(?:"([^"]*)"|'([^']*)')`, "i"),
  );
  if (!m) return undefined;
  return decodeEntities(m[1] ?? m[2] ?? "").trim() || undefined;
};

const toCount = (v: string | undefined): number | undefined => {
  if (v === undefined || !/^\d+$/.test(v)) return undefined;
  return Number(v);
};

const toNumber = (v: string | undefined): number | undefined => {
  if (v === undefined || !/^\d+(\.\d+)?$/.test(v)) return undefined;
  return Number(v);
};

/* ---------- 轻量解析（列表展示） ---------- */

export function parseShowNfo(input: string): ShowNfo | null {
  const xml = normalize(input);
  const title = tag(xml, "title");
  if (!title) return null;
  // 嵌套 <rating name=".."><value>8.5</value> 优先，兼容老式平铺 <rating>8.5</rating>
  const nested = xml.match(
    /<rating[\s>][^>]*>\s*<value>\s*([\d.]+)\s*<\/value>/i,
  );
  const raw = nested?.[1] ?? tag(xml, "rating");
  return {
    title,
    premiered: tag(xml, "premiered"),
    plot: tag(xml, "plot"),
    rating: toNumber(raw),
    genres: allTags(xml, "genre"),
    seasonCount: toCount(tag(xml, "season")),
    episodeCount: toCount(tag(xml, "episode")),
  };
}

export function parseEpisodeNfo(input: string): EpisodeNfo {
  const xml = normalize(input);
  const runtime = tag(xml, "runtime");
  const runtimeMinutes =
    runtime !== undefined && /^\d+(\.\d+)?$/.test(runtime)
      ? Math.round(Number(runtime))
      : undefined;
  return { title: tag(xml, "title"), aired: tag(xml, "aired"), runtimeMinutes };
}

/* ---------- 完整解析（编辑回读） ---------- */

function parseRatings(xml: string): Rating[] {
  const container = blocks(xml, "ratings")[0];
  const out: Rating[] = [];
  for (const r of blocks(container?.inner ?? "", "rating")) {
    const source = attr(r.attrs, "name");
    // Kodi 禁止 name="default"；无来源的评分无法写回合法 NFO
    if (source === undefined || source.toLowerCase() === "default") continue;
    const value = toNumber(tag(r.inner, "value"));
    if (value === undefined) continue;
    out.push({ source, value, votes: toCount(tag(r.inner, "votes")) });
  }
  if (out.length === 0) {
    const flat = toNumber(tag(xml, "rating"));
    if (flat !== undefined) out.push({ source: "imdb", value: flat });
  }
  return out;
}

function parseUniqueIds(xml: string): UniqueId[] {
  const out: UniqueId[] = [];
  for (const b of blocks(xml, "uniqueid")) {
    const source = attr(b.attrs, "type");
    const value = decodeEntities(b.inner).trim();
    if (source === undefined || source.toLowerCase() === "default") continue;
    if (value === "") continue;
    out.push({ source, value });
  }
  return out;
}

function parseActors(xml: string): Actor[] {
  const out: Actor[] = [];
  for (const [i, b] of blocks(xml, "actor").entries()) {
    const name = tag(b.inner, "name");
    if (name === undefined) continue;
    out.push({
      name,
      role: tag(b.inner, "role"),
      order: toCount(tag(b.inner, "order")) ?? i,
      thumbUrl: tag(b.inner, "thumb"),
    });
  }
  return out;
}

export function parseMetadataNfo(
  input: string,
  mediaType: MediaType,
): MediaMetadata | null {
  const xml = normalize(input);
  const title = tag(xml, "title");
  if (title === undefined) return null;
  const m = newMetadata(mediaType, title);
  m.originalTitle = tag(xml, "originaltitle");
  m.plot = tag(xml, "plot");
  m.premiered = tag(xml, "premiered");
  m.status = tag(xml, "status");
  m.genres = allTags(xml, "genre");
  m.studios = allTags(xml, "studio");
  m.ratings = parseRatings(xml);
  m.uniqueIds = parseUniqueIds(xml);
  m.actors = parseActors(xml);
  m.seasonCount = toCount(tag(xml, "season"));
  m.episodeCount = toCount(tag(xml, "episode"));
  return m;
}
