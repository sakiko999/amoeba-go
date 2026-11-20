/**
 * vite 配置：React 19 + Tailwind 4 + @demo/ui。纯 renderer 单页，无 main/preload。
 *
 * 共享层：pack 时（PAGE_VENDOR_EXTERNAL=1 且 page-vendor manifest 存在）把
 * react/@demo/ui external 掉，import map + CSP hash 由插件构建期注入 index.html
 * （vendor://）；未构建共享层时全内联。
 */

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";
import {
  injectIntoHtmlText,
  readVendorManifest,
  type VendorManifest,
} from "../../web/page-vendor/scripts/import-map";

const SHARED_EXTERNAL = [
  "react",
  "react/jsx-runtime",
  "react-dom",
  "react-dom/client",
  "@demo/ui",
];

const vendorManifest =
  process.env.PAGE_VENDOR_EXTERNAL === "1"
    ? readVendorManifest(
        resolve(__dirname, "..", "..", "web", "page-vendor", "out", "dist", "manifest.json"),
      )
    : undefined;

function vendorImportMapPlugin(manifest: VendorManifest) {
  return {
    name: "vendor-import-map",
    apply: "build" as const,
    transformIndexHtml(html: string): string {
      return injectIntoHtmlText(html, manifest);
    },
  };
}

export default defineConfig({
  root: "src/renderer",
  // 页面经 page://<id>/renderer/index.html 服务,资源必须相对引用;默认的
  // 绝对 /assets/... 会落到组件根而 404(WebView2 报 MIME text/plain)
  base: "./",
  build: {
    // 直接产出组件包结构(out/page/renderer + component.json),宿主 pages 目录
    // 指向 out/ 即可,不需要单独的投影/同步步骤
    outDir: resolve(__dirname, "out/page/renderer"),
    emptyOutDir: true,
    rollupOptions: {
      input: resolve(__dirname, "src/renderer/index.html"),
      ...(vendorManifest !== undefined ? { external: SHARED_EXTERNAL } : {}),
    },
  },
  plugins: [
    react(),
    tailwindcss(),
    ...(vendorManifest !== undefined
      ? [vendorImportMapPlugin(vendorManifest)]
      : []),
  ],
});
