/**
 * import map 工具：由 vendor manifest 生成页面/宿主 import map 并注入 HTML。
 * 各页面与宿主的 electron-vite 插件（transformIndexHtml）都从这里 import。
 *
 * HTML 源里放一个**合法空 map** 作占位（`{"imports":{}}`）——dev 下未注入时
 * 浏览器不会因非法 JSON 报错；构建时整段替换为真实 map。
 * 内联 importmap 受 CSP script-src 管辖，注入时同步追加其 sha256 hash source
 * （锚点 `script-src 'self' vendor:`），无需 'unsafe-inline'。
 */

import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";

export interface VendorManifest {
  baseUrl: string;
  versions: Record<string, string>;
  entries: Record<string, string>;
}

/** 页面构建的 external 集合（与 manifest 的 entry 一一对应） */
export const SHARED_SPECIFIERS = [
  "react",
  "react/jsx-runtime",
  "react-dom",
  "react-dom/client",
  "@demo/ui",
] as const;

/** HTML 里 importmap 占位（合法空 map；构建时整段替换） */
export const IMPORT_MAP_PLACEHOLDER =
  '<script type="importmap">{"imports":{}}</script>';
const CSP_ANCHOR = "script-src 'self' vendor:";

export function importMapOf(manifest: VendorManifest): Record<string, string> {
  const { baseUrl: base, entries: e } = manifest;
  if (
    e.react === undefined ||
    e["jsx-runtime"] === undefined ||
    e["react-dom"] === undefined ||
    e["demo-ui"] === undefined
  ) {
    throw new Error("vendor manifest 缺少 entry（先重跑 page-vendor build）");
  }
  return {
    react: base + e.react,
    "react/jsx-runtime": base + e["jsx-runtime"],
    "react-dom": base + e["react-dom"],
    // react-dom 的根与 /client 是同一个 entry（同一实例、两个导出面）
    "react-dom/client": base + e["react-dom"],
    "@demo/ui": base + e["demo-ui"],
  };
}

export function readVendorManifest(path: string): VendorManifest | undefined {
  try {
    return JSON.parse(readFileSync(path, "utf8")) as VendorManifest;
  } catch {
    return undefined;
  }
}

/** 替换 HTML 中的 importmap 占位并给 CSP 追加该 script 块的 hash source。 */
export function injectIntoHtmlText(
  html: string,
  manifest: VendorManifest,
): string {
  if (!html.includes(IMPORT_MAP_PLACEHOLDER)) {
    throw new Error(`HTML 缺少 import map 占位 ${IMPORT_MAP_PLACEHOLDER}`);
  }
  if (!html.includes(CSP_ANCHOR)) {
    throw new Error(`HTML 的 CSP 缺少锚点 ${CSP_ANCHOR}`);
  }
  const mapJson = JSON.stringify({ imports: importMapOf(manifest) });
  // CSP hash 覆盖 script 元素的文本内容（即注入后的 JSON 本身）
  const hash = createHash("sha256").update(mapJson).digest("base64");
  return html
    .replace(
      IMPORT_MAP_PLACEHOLDER,
      `<script type="importmap">${mapJson}</script>`,
    )
    .replace(CSP_ANCHOR, `${CSP_ANCHOR} 'sha256-${hash}'`);
}
