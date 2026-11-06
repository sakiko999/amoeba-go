/**
 * page:// 协议：host = 页面 id，根 = 页面包目录。
 * 必须按 session 注册（每个页面 tab 的 persist:page-<id> 是独立 session）。
 * 路径校验用 relative + realpath 双重检查，不能只用 startsWith（out/renderer-evil 会绕过）。
 */

import type { Session } from "electron";
import { realpath } from "node:fs/promises";
import { resolve } from "node:path";
import { serveFile } from "../staticFiles";
import { isInside } from "./manifest";

export interface PageRoots {
  rootOf(id: string): string | undefined;
}

const notFound = (): Response => new Response(null, { status: 404 });

export function registerPageProtocol(ses: Session, pages: PageRoots): void {
  ses.protocol.handle("page", async (request) => {
    try {
      const url = new URL(request.url);
      const root = pages.rootOf(url.hostname);
      if (root === undefined) return notFound();

      const rel = decodeURIComponent(url.pathname).replace(/^\/+/, "");
      if (rel === "" || rel.includes("\0")) return notFound();

      const file = resolve(root, rel);
      if (!isInside(root, file)) return notFound();

      // asar 内 realpath 可能不可用；此时 resolve+isInside 已挡目录穿越
      try {
        const realRoot = await realpath(root);
        const real = await realpath(file);
        if (!isInside(realRoot, real)) return notFound();
      } catch {
        // 忽略：打包形态（app.asar）下走上面的 resolve 校验
      }

      return serveFile(file);
    } catch {
      return notFound();
    }
  });
}
