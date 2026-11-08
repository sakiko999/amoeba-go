/**
 * app:// 与 page:// 的静态文件响应：统一用 fs.readFile 而不是 net.fetch(file://)，
 * 因为打包后资源在 app.asar 内（file:// 网络栈读不到 asar，Node fs 可以）。
 */

import { readFile } from "node:fs/promises";
import { extname } from "node:path";

const MIME: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".webp": "image/webp",
  ".gif": "image/gif",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
};

export async function serveFile(
  file: string,
  extraHeaders?: Record<string, string>,
): Promise<Response> {
  try {
    const body = await readFile(file);
    const type =
      MIME[extname(file).toLowerCase()] ?? "application/octet-stream";
    return new Response(body, {
      headers: { "content-type": type, ...extraHeaders },
    });
  } catch {
    return new Response(null, { status: 404 });
  }
}
