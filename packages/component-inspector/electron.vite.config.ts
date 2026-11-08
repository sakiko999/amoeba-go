/**
 * electron-vite 配置：preload 打 Node CJS（electron external），renderer 为
 * React 19 + Tailwind 4 + @demo/ui（与 media-manager 同一套页面形态）。
 * 无 main 构建段：组件检查页没有后端扩展，组件包不带 main.js。
 */

import { defineConfig } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";

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
      rollupOptions: { input: resolve(__dirname, "src/renderer/index.html") },
    },
    plugins: [react(), tailwindcss()],
  },
});
