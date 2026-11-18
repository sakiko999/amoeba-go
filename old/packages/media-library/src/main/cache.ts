/**
 * 元数据缓存：node:sqlite（Electron 45 内置 Node 24，稳定可用）。
 * 按缓存 key 存 GET 响应原文，TTL 7 天；主要为豆瓣限流与减少 TMDB 请求。
 */

import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { app } from "electron";
import { DatabaseSync } from "node:sqlite";

const TTL_MS = 7 * 24 * 60 * 60 * 1000;

let db: DatabaseSync | undefined;
let dbDir: string | undefined;
let configured = false;

/**
 * 指定缓存目录（页面模式下由宿主传入 page-data 目录；独立应用不调用则用 userData）。
 * 切换目录会先关闭现有连接，下次读写时重建。
 */
export function configureCache(dir: string | undefined): void {
  if (configured && dir === dbDir) return;
  closeCache();
  dbDir = dir;
  configured = true;
}

/** 页面扩展 deactivate 时调用：释放 sqlite 句柄，否则重载会累积连接 */
export function closeCache(): void {
  db?.close();
  db = undefined;
}

function open(): DatabaseSync {
  if (db !== undefined) return db;
  const dir = dbDir ?? app.getPath("userData");
  mkdirSync(dir, { recursive: true });
  db = new DatabaseSync(join(dir, "cache.db"));
  db.exec(
    "CREATE TABLE IF NOT EXISTS meta_cache (key TEXT PRIMARY KEY, value TEXT NOT NULL, created_at INTEGER NOT NULL)",
  );
  return db;
}

export function cacheGet(key: string): string | undefined {
  const row = open()
    .prepare("SELECT value, created_at FROM meta_cache WHERE key = ?")
    .get(key) as { value: string; created_at: number } | undefined;
  if (row === undefined) return undefined;
  if (Date.now() - row.created_at > TTL_MS) return undefined;
  return row.value;
}

export function cacheSet(key: string, value: string): void {
  open()
    .prepare(
      "INSERT OR REPLACE INTO meta_cache (key, value, created_at) VALUES (?, ?, ?)",
    )
    .run(key, value, Date.now());
}

export function cacheClear(): number {
  const result = open().prepare("DELETE FROM meta_cache").run();
  return Number(result.changes);
}

/** 缓存 key：路径段拼接（与旧 core 的 cache_key 同思路） */
export function cacheKey(parts: readonly string[]): string {
  return parts.join("/");
}
