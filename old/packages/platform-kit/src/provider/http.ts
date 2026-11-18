/**
 * 平台 provider 的 HTTP 基座（中立包，供 platform-media-* 使用）：
 * 统一传输面 + UA + 缓存包装 + 错误类型。
 *
 * 传输优先走 `globalThis.appHost.http`（与数据渠道平台的 runtime/host 同一条
 * 路径，宿主形态由 electron-host 安装、未来共享治理）；独立形态
 * （media-manager app，无 appHost）回退直连 net.fetch。缓存固定落在
 * userData/provider-cache/cache.db（宿主与独立形态一致）。
 *
 * net.fetch 只能在 app ready 后调用（组件 activate 之后的调用满足）。
 */

import { app, net } from "electron";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";

import { NotFoundError, ProviderError } from "@tabs/platform-contracts";

// 平台 provider 只引本模块：契约类型 + 错误类随 provider 基座一起 re-export。
export * from "@tabs/platform-contracts";

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

interface ProviderHttpRequest {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string;
  responseType?: string;
}

/**
 * 统一传输：appHost.http 存在则走宿主（与数据渠道同路径）；否则回退
 * net.fetch（独立形态）。响应形状对齐 runtime/electron-host 的 hostHttpRequest。
 */
async function httpRequest(
  req: ProviderHttpRequest,
): Promise<{ status: number; headers: Record<string, string>; body: unknown }> {
  const host = (
    globalThis as unknown as {
      appHost?: {
        http?: {
          request?: (
            r: ProviderHttpRequest,
          ) => Promise<{ status: number; headers: Record<string, string>; body: unknown }>;
        };
      };
    }
  ).appHost?.http;
  if (typeof host?.request === "function") {
    return host.request(req);
  }
  const res = await net.fetch(req.url, {
    method: req.method ?? "GET",
    headers: { "User-Agent": USER_AGENT, ...req.headers },
    ...(req.body !== undefined ? { body: req.body } : {}),
  });
  const rawSetCookie = res.headers.getSetCookie?.() ?? [];
  const headers: Record<string, string> =
    rawSetCookie.length > 0 ? { "set-cookie": rawSetCookie.join("\n") } : {};
  const text = await res.text();
  return {
    status: res.status,
    headers,
    body: req.responseType === "json" ? (text.trim() === "" ? null : JSON.parse(text)) : text,
  };
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
  const resp = await httpRequest({
    url,
    ...(init.method !== undefined ? { method: init.method } : {}),
    ...(init.body !== undefined ? { body: init.body } : {}),
    headers: { "User-Agent": USER_AGENT, Accept: "application/json", ...init.headers },
    responseType: "text",
  });
  if (resp.status === 404) {
    throw new NotFoundError(`${source} 中不存在该条目：${url}`);
  }
  if (resp.status < 200 || resp.status >= 300) {
    throw new ProviderError(
      source,
      `HTTP ${resp.status}: ${String(resp.body).slice(0, 200)}`,
    );
  }
  return String(resp.body);
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
  const text = await request(source, url, { headers });
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
    headers: { "Content-Type": "application/json" },
  });
  cacheSet(key, text);
  return JSON.parse(text);
}
