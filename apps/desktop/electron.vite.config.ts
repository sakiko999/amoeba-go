/**
 * electron-vite 配置：main / preload 打 Node CJS（electron 与内置模块自动 external），
 * renderer 为多页 MPA —— host（tab 条）+ 每个 tab 子包一个 html 入口。
 * renderer root 提到仓库根：packages/* 与 apps/* 都在 root 内，
 * 产物路径 = 源路径相对仓库根（如 packages/tab-clock/src/index.html），
 * 寻址约定见 src/main/renderTarget.ts。
 */

import { defineConfig } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";

const repoRoot = resolve(__dirname, "../..");
const desktopModules = resolve(__dirname, "node_modules");

/**
 * renderer root 是仓库根，vite 的 optimizeDeps 会从 root 解析依赖；react 按“谁用谁声明”
 * 装在 apps/desktop（@demo/ui 是 peer）。这里显式把 react 系列指向本包 node_modules，
 * 否则 dev 报 Failed to resolve dependency: react，且 @demo/ui 的 react 也无法从 packages/ui 向上解析。
 * 同版本在 bun store 里是同一份 realpath，仍是单实例。
 */
const reactAlias = [
  "react/jsx-dev-runtime",
  "react/jsx-runtime",
  "react-dom/client",
  "react-dom",
  "react",
].map((id) => ({ find: id, replacement: resolve(desktopModules, id) }));

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
    resolve: { alias: reactAlias },
    // react 只处理宿主 ui 的 tsx（html 入口与 tab 子包纯 ts 不进 babel）；tailwind 处理宿主 styles.css
    plugins: [
      react({ include: /apps\/desktop\/src\/ui\/.*\.tsx?$/ }),
      tailwindcss(),
    ],
    server: {
      // dev server 就绪时先转换宿主入口，Electron 请求页面时少等一轮按需编译
      warmup: {
        clientFiles: [
          "apps/desktop/src/ui/renderer.tsx",
          "apps/desktop/src/ui/styles.css",
        ],
      },
    },
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
