/**
 * 组件检查页的渲染侧桥：只读组件清单 + 变更通知。
 *
 * 宿主在 ipcMain 注册 COMPONENT_CHANNELS.list（PageRegistry.inspect()），
 * 安装/卸载/重载后向所有 tab 广播 changed。页面不含 main.js（无后端状态）。
 */

import { contextBridge, ipcRenderer } from "electron";
import { COMPONENT_CHANNELS } from "@host/sdk";
import type { ComponentInfo } from "@host/sdk";

const api = {
  list: (): Promise<ComponentInfo[]> =>
    ipcRenderer.invoke(COMPONENT_CHANNELS.list),
  onChanged: (cb: () => void): void => {
    ipcRenderer.on(COMPONENT_CHANNELS.changed, () => cb());
  },
};

contextBridge.exposeInMainWorld("componentInspector", api);
