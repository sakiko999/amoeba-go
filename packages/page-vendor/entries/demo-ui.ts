/**
 * 共享层入口：@demo/ui barrel 的完整导出面。
 * 内部依赖（radix/@base-ui/lucide 图标等）全部内联；只 external react 系
 * （裸名经 import map 解析回共享层，保证 React 单实例）。
 * 代价：比单页 tree-shake 版本大，但全宿主只此一份（N≥3 页即回本）。
 */

export * from "@demo/ui";
