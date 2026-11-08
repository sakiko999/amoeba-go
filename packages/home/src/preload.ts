/**
 * 首页的渲染侧桥：子页面清单 + 打开，加上组件概览（复用检查页的只读数据）。
 *
 * 宿主注册 NAV_CHANNELS.list / NAV_CHANNELS.open（apps/desktop/src/main/index.ts）；
 * 组件数据来自 COMPONENT_CHANNELS（与组件检查页同一份只读 IPC）。
 * 页面不含 main.js（无后端状态）。
 */

import { contextBridge, ipcRenderer } from "electron";
import { COMPONENT_CHANNELS, NAV_CHANNELS } from "@host/sdk";
import type { ComponentInfo, NavEntry } from "@host/sdk";

const api = {
  list: (): Promise<NavEntry[]> => ipcRenderer.invoke(NAV_CHANNELS.list),
  open: (id: string): Promise<boolean> =>
    ipcRenderer.invoke(NAV_CHANNELS.open, id),
  components: (): Promise<ComponentInfo[]> =>
    ipcRenderer.invoke(COMPONENT_CHANNELS.list),
  onComponentsChanged: (cb: () => void): void => {
    ipcRenderer.on(COMPONENT_CHANNELS.changed, () => cb());
  },
};

contextBridge.exposeInMainWorld("homeNav", api);
