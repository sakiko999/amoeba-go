import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// chrome（tab 条）是纯静态产物：全部依赖内联，宿主经 mygo SetFrontend 嵌入后
// 从 mygo://localhost 提供；不用 vendor:// 共享层（单页面，共享无收益）。
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
  build: { outDir: "dist", emptyOutDir: true },
});
