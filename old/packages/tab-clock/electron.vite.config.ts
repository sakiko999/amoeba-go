/**
 * 页面组件构建：只有 renderer（vanilla TS，单 html 入口）。
 * 产物 out/renderer 由 scripts/pack.ts 组装到 out/page/；
 * 无 main/preload：window.tab 由宿主的 pageBridge 叠加提供。
 */
import { defineConfig } from "electron-vite";
import { resolve } from "node:path";

export default defineConfig({
  renderer: {
    root: "src",
    build: {
      outDir: resolve(__dirname, "out/renderer"),
      rolldownOptions: { input: resolve(__dirname, "src/index.html") },
    },
  },
});
