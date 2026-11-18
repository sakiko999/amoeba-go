/**
 * electron-vite 配置：main / preload 打 Node CJS（electron 与内置模块自动 external），
 * renderer 只有 host UI 一个 html 入口（页面组件各自打包，不进宿主构建）。
 * renderer root 提到仓库根，产物路径 = 源路径相对仓库根
 * （apps/desktop/src/ui/index.html），寻址约定见 src/main/renderTarget.ts。
 *
 * 共享层（vendor://）：build 时若 page-vendor 的 manifest 存在，宿主 UI 的
 * react/@demo/ui 走 external（import map 解析到 vendor://，全宿主单实例），
 * 并由插件把 map + CSP hash 注入 host html；manifest 缺失（未跑 pages:prepare）
 * 则回退全内联。dev 不 external（alias 仍指向本包 node_modules）。
 */

import { defineConfig } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "node:path";
import {
  CSP_ANCHOR,
  injectIntoHtmlText,
  readVendorManifest,
} from "../../packages/page-vendor/scripts/import-map";

const repoRoot = resolve(__dirname, "../..");
const desktopModules = resolve(__dirname, "node_modules");
const vendorManifestPath = resolve(
  repoRoot,
  "packages",
  "page-vendor",
  "out",
  "dist",
  "manifest.json",
);

/** pack 页面的共享集（宿主 UI 同形；host 与页面组件共享同一份 vendor） */
const SHARED_EXTERNAL = [
  "react",
  "react/jsx-runtime",
  "react-dom",
  "react-dom/client",
  "@demo/ui",
];

/**
 * renderer root 是仓库根，vite 的 optimizeDeps 会从 root 解析依赖；react 按“谁用谁声明”
 * 装在 apps/desktop（@demo/ui 是 peer）。dev 下把 react 系列指向本包 node_modules，
 * 否则 dev 报 Failed to resolve dependency: react，且 @demo/ui 的 react 也无法从 packages/ui 向上解析。
 * build + vendor 共享层时不能 alias（external 需按裸名匹配），由 import map 解析。
 */
const reactAlias = [
  "react/jsx-dev-runtime",
  "react/jsx-runtime",
  "react-dom/client",
  "react-dom",
  "react",
].map((id) => ({ find: id, replacement: resolve(desktopModules, id) }));

export default defineConfig(({ command }) => {
  const vendorManifest =
    command === "build" ? readVendorManifest(vendorManifestPath) : undefined;

  return {
    main: {
      build: {
        outDir: "out/main",
        rolldownOptions: {
          input: resolve(__dirname, "src/main/index.ts"),
        },
      },
    },
    preload: {
      build: {
        outDir: "out/preload",
        rolldownOptions: {
          input: {
            // host 页面用 index；页面组件由 session 级 pageBridge 叠加
            index: resolve(__dirname, "src/preload/index.ts"),
            pageBridge: resolve(__dirname, "src/preload/pageBridge.ts"),
          },
        },
      },
    },
    renderer: {
      root: repoRoot,
      ...(command === "serve" ? { resolve: { alias: reactAlias } } : {}),
      // react 只处理宿主 ui 的 tsx；tailwind 处理宿主 styles.css
      plugins: [
        react({ include: /apps\/desktop\/src\/ui\/.*\.tsx?$/ }),
        tailwindcss(),
        ...(vendorManifest !== undefined
          ? [
              {
                name: "vendor-import-map",
                apply: "build" as const,
                transformIndexHtml(html: string): string {
                  // 只处理带共享层 CSP 的入口（当前只有 host ui）
                  return html.includes(CSP_ANCHOR)
                    ? injectIntoHtmlText(html, vendorManifest)
                    : html;
                },
              },
            ]
          : []),
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
        rolldownOptions: {
          input: {
            "ui/index": resolve(__dirname, "src/ui/index.html"),
          },
          ...(vendorManifest !== undefined
            ? { external: SHARED_EXTERNAL }
            : {}),
        },
      },
    },
  };
});
