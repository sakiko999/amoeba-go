/**
 * 上游 RSS/Atom 解析（仅输入方向：YouTube 官方 feed；本平台专属，随包分发）。
 *
 * 平台接口输出的是结构化数据，不做 XML 序列化；这里只把上游 XML
 * 归一成 ParsedFeed/ParsedItem 供本平台映射成 Item。
 */

import { XMLParser } from "fast-xml-parser";

const parser = new XMLParser({
  ignoreAttributes: false,
  attributeNamePrefix: "@_",
  // 关闭文本自动转数值：数字形字符串（roomId "007"、guid "0123"）必须保持字符串。
  parseTagValue: false,
});

export interface ParsedFeed {
  channel: {
    title?: string;
    link?: string;
    description?: string;
    item: ParsedItem[];
    /** 原始 channel 节点（平台可读自定义扩展）。 */
    raw?: Record<string, unknown>;
  };
}

export interface ParsedItem {
  title?: string;
  link?: string;
  description?: string;
  content?: string;
  pubDate?: string;
  author?: string;
  guid?: string;
  /** 原始解析节点，可从中抠自定义字段（yt:videoId / media:group）。 */
  raw?: Record<string, unknown>;
}

function ensureArray<T>(val: T | T[] | undefined): T[] {
  if (val === undefined || val === null) return [];
  return Array.isArray(val) ? val : [val];
}

function asLink(val: unknown): string {
  const pick = (v: unknown): string => {
    if (typeof v === "string") return v;
    if (v !== null && typeof v === "object") {
      const o = v as Record<string, unknown>;
      if (typeof o["@_href"] === "string") return o["@_href"] as string;
      if (typeof o["#text"] === "string") return o["#text"] as string;
    }
    return "";
  };
  // Atom 允许一条 entry 多个 <link>（rel="alternate"/"self"/"enclosure"…）。
  // 取 rel="alternate"（条目网页地址）优先，其次无 rel 的，最后才将就第一个有 href 的。
  if (Array.isArray(val)) {
    const objs = val.filter(
      (v): v is Record<string, unknown> => !!v && typeof v === "object",
    );
    const alt =
      objs.find((o) => o["@_rel"] === "alternate") ??
      objs.find((o) => o["@_rel"] === undefined);
    return (alt ? pick(alt) : "") || val.map(pick).find((s) => s) || "";
  }
  return pick(val);
}

function asAuthor(val: unknown): string {
  if (typeof val === "string") return val;
  if (val !== null && typeof val === "object") {
    const o = val as Record<string, unknown>;
    const name = o["name"] ?? o["@_name"];
    if (typeof name === "string") return name;
  }
  return "";
}

export function parseFeed(xml: string): ParsedFeed {
  const raw = parser.parse(xml) as Record<string, unknown>;
  const root = (raw.rss ?? raw.feed ?? raw) as Record<string, unknown>;

  const channel = root.channel as Record<string, unknown> | undefined;
  if (channel) {
    return {
      channel: {
        title: asString(channel.title),
        link: asString(channel.link),
        description: asString(channel.description),
        item: ensureArray(
          channel.item as ParsedItem | ParsedItem[] | undefined,
        ).map(normalizeItem),
        raw: channel,
      },
    };
  }

  const entries = ensureArray(
    root.entry as Record<string, unknown> | Record<string, unknown>[] | undefined,
  );
  return {
    channel: {
      title: asString(root.title),
      link: asLink(root.link),
      description: asString(root.subtitle),
      item: entries.map((e) => normalizeAtomEntry(e as Record<string, unknown>)),
    },
  };
}

function normalizeItem(item: ParsedItem): ParsedItem {
  return {
    title: asString(item.title),
    link: asLink(item.link),
    description: asString(item.description),
    content: asString(item["content:encoded" as keyof ParsedItem] ?? item.content),
    pubDate: asString(item.pubDate),
    author: asAuthor(item.author),
    guid: asString(item.guid),
    raw: item as Record<string, unknown>,
  };
}

function normalizeAtomEntry(e: Record<string, unknown>): ParsedItem {
  return {
    title: asString(e.title),
    link: asLink(e.link),
    description: asString(e.summary ?? e.content),
    content: asString(e.content ?? e.summary),
    pubDate: asString(e.published ?? e.updated),
    author: asAuthor(e.author),
    guid: asString(e.id),
    raw: e,
  };
}

function asString(val: unknown): string | undefined {
  if (val === undefined || val === null) return undefined;
  if (typeof val === "string") return val;
  if (typeof val === "number" || typeof val === "boolean") return String(val);
  if (typeof val === "object") {
    const o = val as Record<string, unknown>;
    const text = o["#text"];
    if (typeof text === "string") return text;
    if (text !== undefined) return asString(text);
  }
  return undefined;
}
