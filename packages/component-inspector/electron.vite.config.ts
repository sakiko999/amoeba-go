/**
 * electron-vite 配置：preload 打 Node CJS（electron external），renderer 为
 * React 19 + Tailwind 4 + @demo/ui（与 media-manager 同一套页面形态）。
 * 无 main 构建段：组件检查页没有后端扩展，组件包不带 main.js。
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
  preload: {
    build: {
      outDir: "out/preload",
      rolldownOptions: { input: resolve(__dirname, "src/preload/index.ts") },
    },
  },
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
