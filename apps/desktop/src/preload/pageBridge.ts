/**
 * runtime 页面的宿主 bridge：由 host 通过 session.registerPreloadScript 叠加到页面，
 * 不替换页面包自带的 preload（页面仍可注入 window.media 等自己的桥）。
 * 暴露：
 * - window.tab：页面实例身份 + setTitle/close/ping/systemInfo + HostState 订阅
 * - window.hostState：首帧初始状态 + onChanged（保持与 media-manager 既有桥一致）
 * 页面自有 preload 不应再注入同名全局（media 已按 whoami 条件注入）。
 */

import { contextBridge, ipcRenderer } from "electron";
import type { HostState, TabApi, Theme, Whoami } from "@host/sdk";

const whoami = ipcRenderer.sendSync("app:whoami") as Whoami;
if (whoami.role === "tab") {
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
    state: () => ipcRenderer.invoke("host:state:get"),
    onStateChanged: (cb) => {
      ipcRenderer.on("host:state", (_e, state: HostState) => cb(state));
    },
    theme: () => ipcRenderer.invoke("theme:get"),
    onThemeChanged: (cb) => {
      ipcRenderer.on("theme:changed", (_e, theme: Theme) => cb(theme));
    },
  };
  contextBridge.exposeInMainWorld("tab", tab);

  const initial = (ipcRenderer.sendSync("host:state:sync") as
    HostState | undefined) ?? { theme: "dark" };
  contextBridge.exposeInMainWorld("hostState", {
    initial,
    onChanged: (cb: (state: HostState) => void) => {
      ipcRenderer.on("host:state", (_e, state: HostState) => cb(state));
    },
  });
}
