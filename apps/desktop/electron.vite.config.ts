/**
 * electron-vite 配置：main / preload 打 Node CJS（electron 与内置模块自动 external），
 * renderer 为多页 MPA —— host（tab 条）+ 每个 tab 子包一个 html 入口。
 * renderer root 提到仓库根：packages/* 与 apps/* 都在 root 内，
 * 产物路径 = 源路径相对仓库根（如 packages/tab-clock/src/index.html），
 * 寻址约定见 src/main/renderTarget.ts。
 */

import { defineConfig } from "electron-vite";
import { resolve } from "node:path";

const repoRoot = resolve(__dirname, "../..");

export default defineConfig({
  main: {
    build: {
      outDir: "out/main",
      rollupOptions: {
        input: resolve(__dirname, "src/main/index.ts"),
      },
    },
  },
  preload: {
    build: {
      outDir: "out/preload",
      rollupOptions: {
        input: {
          // 宿主页面与静态 tab 用 index；runtime 页面用 pageBridge 叠加
          index: resolve(__dirname, "src/preload/index.ts"),
          pageBridge: resolve(__dirname, "src/preload/pageBridge.ts"),
        },
      },
    },
  },
  renderer: {
    root: repoRoot,
    build: {
      outDir: resolve(__dirname, "out/renderer"),
      rollupOptions: {
        input: {
          "ui/index": resolve(__dirname, "src/ui/index.html"),
          "tabs/clock/index": resolve(
            repoRoot,
            "packages/tab-clock/src/index.html",
          ),
          "tabs/notes/index": resolve(
            repoRoot,
            "packages/tab-notes/src/index.html",
          ),
          "tabs/sysinfo/index": resolve(
            repoRoot,
            "packages/tab-sysinfo/src/index.html",
          ),
        },
      },
    },
  },
});
