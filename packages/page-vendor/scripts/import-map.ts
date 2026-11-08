/**
 * import map 工具：由 vendor manifest 生成页面/宿主 import map 并注入 HTML。
 * 各页面与宿主的 electron-vite 插件（transformIndexHtml）都从这里 import。
 *
 * 源 HTML 不含任何 import map 占位：构建时把 `<script type="importmap">` 插在
 * CSP meta 之后（必须早于所有 module script），并同步给 CSP 追加该内联 map 的
 * sha256 hash source（锚点 `script-src 'self' vendor:`），无需 'unsafe-inline'。
 * dev 不注入（Vite 自己解析裸导入），因此没有内联脚本，也没有 hash 需求。
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

/** CSP 里必须存在的锚点（注入 hash 的位置） */
export const CSP_ANCHOR = "script-src 'self' vendor:";
/** 注入点：CSP meta 之后（保证 map 早于 module script，且受该策略管辖） */
const CSP_META = /<meta\s+http-equiv="Content-Security-Policy"[^>]*>/i;

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

/** 在 CSP meta 之后插入 import map，并给 CSP 追加该 inline script 的 hash。 */
export function injectIntoHtmlText(
  html: string,
  manifest: VendorManifest,
): string {
  if (!html.includes(CSP_ANCHOR)) {
    throw new Error(`HTML 的 CSP 缺少锚点 ${CSP_ANCHOR}`);
  }
  if (!CSP_META.test(html)) {
    throw new Error("HTML 缺少 CSP meta，无法确定 import map 的插入点");
  }
  const mapJson = JSON.stringify({ imports: importMapOf(manifest) });
  // CSP hash 覆盖 script 元素的文本内容（即注入后的 JSON 本身）
  const hash = createHash("sha256").update(mapJson).digest("base64");
  const withHash = html.replace(CSP_ANCHOR, `${CSP_ANCHOR} 'sha256-${hash}'`);
  return withHash.replace(
    CSP_META,
    (meta) => `${meta}\n    <script type="importmap">${mapJson}</script>`,
  );
}
