/**
 * vendor:// —— 项目级 renderer 共享层（out/vendor，由 packages/page-vendor 产出）。
 * 页面组件与宿主 UI 的 import map 把 react / @demo/ui 等裸模块名解析到这里，
 * 全宿主单实例（同一 URL = 同一模块）。只读、防目录穿越；跨 origin 的 ESM
 * import 需要 ACAO 头（scheme 已按 corsEnabled 注册）。
 */

import { app } from "electron";
import type { Session } from "electron";
import { realpath } from "node:fs/promises";
import { join, resolve } from "node:path";
import { serveFile } from "./staticFiles";
import { isInside } from "./pages/manifest";

const notFound = (): Response => new Response(null, { status: 404 });

export function registerVendorProtocol(ses: Session): void {
  ses.protocol.handle("vendor", async (request) => {
    try {
      const url = new URL(request.url);
      // host 固定为 "app"（standard scheme 需要 host，URL 形态保持稳定）
      const rel = decodeURIComponent(url.pathname).replace(/^\/+/, "");
      if (rel === "" || rel.includes("\0")) return notFound();
      const root = join(app.getAppPath(), "out", "vendor");
      const file = resolve(root, rel);
      if (!isInside(root, file)) return notFound();
      try {
        const realRoot = await realpath(root);
        const real = await realpath(file);
        if (!isInside(realRoot, real)) return notFound();
      } catch {
        // asar 内 realpath 可能不可用；resolve + isInside 已挡目录穿越
      }
      return serveFile(file, { "access-control-allow-origin": "*" });
    } catch {
      return notFound();
    }
  });
}
