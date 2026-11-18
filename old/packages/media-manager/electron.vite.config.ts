/**
 * electron-vite 配置：main / preload 打 Node CJS（electron 与内置模块自动 external），
 * renderer 为 React 19 + Tailwind 4（@demo/ui 公共组件库）。
 * renderer root = src/renderer；生产经 app:// 协议映射 out/renderer（见 src/main/index.ts）。
 *
 * 共享层：宿主页面包（pack:page）时 PAGE_VENDOR_EXTERNAL=1 且 manifest 存在 →
 * react/@demo/ui external，插件在构建期注入 import map + CSP hash（vendor://）；
 * 独立应用 build / dev 保持全内联。
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
  main: {
    build: {
      outDir: "out/main",
      rolldownOptions: { input: resolve(__dirname, "src/main/index.ts") },
    },
  },
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
