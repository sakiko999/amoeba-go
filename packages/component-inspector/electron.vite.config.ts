/**
 * electron-vite 配置：preload 打 Node CJS（electron external），renderer 为
 * React 19 + Tailwind 4 + @demo/ui（与 media-manager 同一套页面形态）。
 * 无 main 构建段：组件检查页没有后端扩展，组件包不带 main.js。
 */

import { defineConfig } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";

/** pack 时置 1：react/@demo/ui 走 vendor:// 共享层（import map 解析）；dev/独立构建全内联 */
const vendorExternal = process.env.PAGE_VENDOR_EXTERNAL === "1";
const SHARED_EXTERNAL = [
  "react",
  "react/jsx-runtime",
  "react-dom",
  "react-dom/client",
  "@demo/ui",
];

export default defineConfig({
  preload: {
    build: {
      outDir: "out/preload",
      rollupOptions: { input: resolve(__dirname, "src/preload/index.ts") },
    },
  },
  renderer: {
    root: "src/renderer",
    build: {
      outDir: resolve(__dirname, "out/renderer"),
      rollupOptions: {
        input: resolve(__dirname, "src/renderer/index.html"),
        ...(vendorExternal ? { external: SHARED_EXTERNAL } : {}),
      },
    },
    plugins: [react(), tailwindcss()],
  },
});
