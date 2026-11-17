/**
 * electron-vite 配置：renderer 为 React 19 + Tailwind 4 + @demo/ui（与 home 同一
 * 页面形态）。无 main / preload 构建段：本页是纯展示页，全部数据经宿主
 * pageBridge 的 window.mygo 调 "Search.*" 服务，组件包不带 main.js / preload。
 *
 * 共享层：pack 时（PAGE_VENDOR_EXTERNAL=1 且 page-vendor manifest 存在）把
 * react/@demo/ui external 掉，并由插件在构建期把 import map + CSP hash 注入
 * index.html（vendor://）；dev / 独立构建保持全内联。
 */

import { defineConfig } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";
import {
  injectIntoHtmlText,
  readVendorManifest,
  type VendorManifest,
} from "../page-vendor/scripts/import-map";

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
        resolve(__dirname, "..", "page-vendor", "out", "dist", "manifest.json"),
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
  renderer: {
    root: "src/renderer",
    build: {
      outDir: resolve(__dirname, "out/renderer"),
      rolldownOptions: {
        input: resolve(__dirname, "src/renderer/index.html"),
        ...(vendorManifest !== undefined
          ? { external: SHARED_EXTERNAL }
          : {}),
      },
    },
    plugins: [
      react(),
      tailwindcss(),
      ...(vendorManifest !== undefined
        ? [vendorImportMapPlugin(vendorManifest)]
        : []),
    ],
  },
});
