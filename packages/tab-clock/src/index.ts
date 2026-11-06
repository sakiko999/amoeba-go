import type { TabManifest } from "@host/sdk";

/** 本 tab 子包的清单：shell 的 registry.ts 导入它；"+" 菜单与构建产物目录都由它派生。 */
export const manifest: TabManifest = {
  id: "clock",
  title: "Clock",
  color: "#7aa2f7",
};
