/**
 * provider 公共 HTTP 基座：net.fetch（走系统证书，主进程专用）+ UA + 缓存包装 + 错误类型。
 * net.fetch 只能在 app ready 后调用（IPC handler 内满足）。
 */

import { net } from "electron";
import { cacheGet, cacheKey, cacheSet } from "../cache";

export const USER_AGENT =
  "media-manager/2.0 (https://github.com/sakiko999/media-manager)";

export class ProviderError extends Error {
  constructor(
    public readonly source: string,
    message: string,
  ) {
    super(`[${source}] ${message}`);
    this.name = "ProviderError";
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}

async function request(
  source: string,
  url: string,
  init: {
    method?: string;
    body?: string;
    headers?: Record<string, string>;
  } = {},
): Promise<string> {
  const resp = await net.fetch(url, {
    method: init.method ?? "GET",
    headers: { "User-Agent": USER_AGENT, ...init.headers },
    body: init.body,
  });
  const text = await resp.text();
  if (resp.status === 404) {
    throw new NotFoundError(`${source} 中不存在该条目：${url}`);
  }
  if (!resp.ok) {
    throw new ProviderError(
      source,
      `HTTP ${resp.status}: ${text.slice(0, 200)}`,
    );
  }
  return text;
}

/** GET JSON（带缓存；headers 供 TMDB 的 Authorization 用） */
export async function getJsonCached(
  source: string,
  keyParts: readonly string[],
  url: string,
  headers?: Record<string, string>,
): Promise<unknown> {
  const key = cacheKey(keyParts);
  const cached = cacheGet(key);
  if (cached !== undefined) return JSON.parse(cached);
  const text = await request(source, url, {
    headers: { Accept: "application/json", ...headers },
  });
  cacheSet(key, text);
  return JSON.parse(text);
}

/** POST JSON（带缓存；Bangumi v0 搜索是 POST + JSON body） */
export async function postJsonCached(
  source: string,
  keyParts: readonly string[],
  url: string,
  body: unknown,
): Promise<unknown> {
  const key = cacheKey(keyParts);
  const cached = cacheGet(key);
  if (cached !== undefined) return JSON.parse(cached);
  const text = await request(source, url, {
    method: "POST",
    body: JSON.stringify(body),
    headers: { Accept: "application/json", "Content-Type": "application/json" },
  });
  cacheSet(key, text);
  return JSON.parse(text);
}

/** 二进制下载（海报）——不走缓存，直接返回 Buffer */
export async function fetchBinary(url: string): Promise<Buffer> {
  const resp = await net.fetch(url, { headers: { "User-Agent": USER_AGENT } });
  if (!resp.ok)
    throw new ProviderError("download", `HTTP ${resp.status}: ${url}`);
  return Buffer.from(await resp.arrayBuffer());
}
