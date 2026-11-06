/**
 * 唯一的 preload：host 页面 → window.host，tab 页面 → window.tab，未知角色不注入任何桥。
 * 角色靠 sendSync("app:whoami") 问主进程（sender 就是自己的 webContents）。
 * sandbox: true 下 preload 只能用 electron 的受限子集（contextBridge / ipcRenderer），正好够用。
 */

import { contextBridge, ipcRenderer } from "electron";
import type { HostApi, TabApi, TabSummary, Theme, Whoami } from "@tabs/sdk";

const whoami = ipcRenderer.sendSync("app:whoami") as Whoami;

if (whoami.role === "host") {
  const host: HostApi = {
    listKinds: () => ipcRenderer.invoke("tabs:list-kinds"),
    createTab: (kind) =>
      ipcRenderer.invoke("tabs:create", kind).then(() => undefined),
    openNewTabMenu: () => ipcRenderer.send("tabs:new-tab-menu"),
    openTabContextMenu: (instanceId) =>
      ipcRenderer.send("tabs:context-menu", instanceId),
    toggleTheme: () => ipcRenderer.send("theme:toggle"),
    openDebugMenu: () => ipcRenderer.send("debug:menu"),
    theme: () => ipcRenderer.invoke("theme:get"),
    onThemeChanged: (cb) => {
      ipcRenderer.on("theme:changed", (_e, theme: Theme) => cb(theme));
    },
    activateTab: (instanceId) => ipcRenderer.send("tabs:activate", instanceId),
    closeTab: (instanceId) => ipcRenderer.send("tabs:close", instanceId),
    onTabsChanged: (cb) => {
      ipcRenderer.on("tabs:changed", (_e, tabs: TabSummary[]) => cb(tabs));
    },
    onKindsChanged: (cb) => {
      ipcRenderer.on("tabs:kinds-changed", () => cb());
    },
  };
  contextBridge.exposeInMainWorld("host", host);
} else if (whoami.role === "tab") {
  const tab: TabApi = {
    instanceId: whoami.instanceId,
    kind: whoami.kind,
    setTitle: (title) => {
      void ipcRenderer.invoke("tab:set-title", title);
    },
    close: () => {
      void ipcRenderer.invoke("tab:close-self");
    },
    ping: (value) => ipcRenderer.invoke("tab:ping", value),
    systemInfo: () => ipcRenderer.invoke("tab:system-info"),
    theme: () => ipcRenderer.invoke("theme:get"),
    onThemeChanged: (cb) => {
      ipcRenderer.on("theme:changed", (_e, theme: Theme) => cb(theme));
    },
  };
  contextBridge.exposeInMainWorld("tab", tab);
}
