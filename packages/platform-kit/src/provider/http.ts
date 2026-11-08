/**
 * 平台 provider 的 HTTP 基座（中立包，供 platform-media-* 使用）：
 * net.fetch（走系统证书，主进程专用）+ UA + 缓存包装 + 错误类型。
 *
 * 与 media-library 的域实现解耦：平台插件只依赖本文件与
 * @tabs/platform-contracts，不依赖 media-library。缓存固定落在
 * userData/provider-cache/cache.db（宿主与独立形态一致）。
 *
 * net.fetch 只能在 app ready 后调用（组件 activate 之后的调用满足）。
 */

import { app, net } from "electron";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";

import { NotFoundError, ProviderError } from "@tabs/platform-contracts";

export { NotFoundError, ProviderError } from "@tabs/platform-contracts";

export const USER_AGENT =
  "media-manager/2.0 (https://github.com/sakiko999/media-manager)";

const TTL_MS = 7 * 24 * 60 * 60 * 1000;

let db: DatabaseSync | undefined;

function openCache(): DatabaseSync {
  if (db !== undefined) return db;
  const dir = join(app.getPath("userData"), "provider-cache");
  mkdirSync(dir, { recursive: true });
  db = new DatabaseSync(join(dir, "cache.db"));
  db.exec(
    "CREATE TABLE IF NOT EXISTS meta_cache (key TEXT PRIMARY KEY, value TEXT NOT NULL, created_at INTEGER NOT NULL)",
  );
  return db;
}

export function cacheKey(parts: readonly string[]): string {
  return parts.join("/");
}

function cacheGet(key: string): string | undefined {
  const row = openCache()
    .prepare("SELECT value, created_at FROM meta_cache WHERE key = ?")
    .get(key) as { value: string; created_at: number } | undefined;
  if (row === undefined) return undefined;
  if (Date.now() - row.created_at > TTL_MS) return undefined;
  return row.value;
}

function cacheSet(key: string, value: string): void {
  openCache()
    .prepare(
      "INSERT OR REPLACE INTO meta_cache (key, value, created_at) VALUES (?, ?, ?)",
    )
    .run(key, value, Date.now());
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
