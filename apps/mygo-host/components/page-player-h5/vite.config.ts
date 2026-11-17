/**
 * vite 配置：React 19 + Tailwind 4，纯 renderer 单页（与 page-search 同构）。
 *
 * 共享层：pack 时（PAGE_VENDOR_EXTERNAL=1 且 page-vendor manifest 存在）把
 * react/@demo/ui external 掉，import map + CSP hash 由插件构建期注入 index.html
 * （vendor://）；未构建共享层时全内联。hls.js/dashjs/flv.js 体积大，一律进
 * 页面自己的 chunk（按需动态 import，见 player/hooks/useMediaStream.ts）。
 */

import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import {
  injectIntoHtmlText,
  readVendorManifest,
  type VendorManifest,
} from "../../../../packages/page-vendor/scripts/import-map";

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
        resolve(
          __dirname,
          "..",
          "..",
          "..",
          "..",
          "packages",
          "page-vendor",
          "out",
          "dist",
          "manifest.json",
        ),
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
  // 页面经 page://<id>/renderer/index.html 服务，资源必须相对引用
  base: "./",
  build: {
    // 直接产出组件包结构（out/page/renderer + component.json）
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
