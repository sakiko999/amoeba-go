/**
 * preload：只注入 window.media。
 * sandbox: true 下只能用 electron 的受限子集（contextBridge / ipcRenderer），正好够用。
 * 写能力（刮削/写 NFO/整理）全部是显式方法，由用户动作调用。
 */

import { contextBridge, ipcRenderer } from "electron";
import { HOST_STATE_CHANNELS } from "@host/sdk";
import type { HostState } from "@host/sdk";
import type { HostStateBridge, MediaApi } from "@tabs/media-library";

const media: MediaApi = {
  pickLibrary: () => ipcRenderer.invoke("library:pick"),
  scanLibrary: (root) => ipcRenderer.invoke("library:scan", root),
  showDetail: (dir) => ipcRenderer.invoke("show:detail", dir),
  imageDataUrl: (path) => ipcRenderer.invoke("image:data-url", path),
  reveal: (path) => {
    ipcRenderer.send("shell:reveal", path);
  },

  searchMetadata: (source, keyword, mediaType) =>
    ipcRenderer.invoke("search:metadata", source, keyword, mediaType),
  scrapePreview: (source, id, mediaType) =>
    ipcRenderer.invoke("scrape:preview", source, id, mediaType),
  scrapeApply: (dir, source, id, mediaType) =>
    ipcRenderer.invoke("scrape:apply", dir, source, id, mediaType),

  readNfo: (dir) => ipcRenderer.invoke("nfo:read", dir),
  writeNfo: (dir, metadata) => ipcRenderer.invoke("nfo:write", dir, metadata),

  planOrganize: (root, dir) => ipcRenderer.invoke("organize:plan", root, dir),
  applyOrganize: (root, dir) => ipcRenderer.invoke("organize:apply", root, dir),
  clearCache: () => ipcRenderer.invoke("cache:clear"),
};

contextBridge.exposeInMainWorld("media", media);

// 嵌入宿主页面时，host 的 pageBridge 已注入 hostState；只有独立应用自己注入。
// app:whoami 在宿主返回 role=tab，独立应用由 main/index.ts 返回 role=unknown。
const whoami = ipcRenderer.sendSync("app:whoami") as
  { role?: string } | undefined;
if (whoami?.role !== "tab") {
  const initial = (ipcRenderer.sendSync(HOST_STATE_CHANNELS.sync) as
    HostState | undefined) ?? { theme: "dark" };
  const hostState: HostStateBridge = {
    initial,
    onChanged: (cb) => {
      ipcRenderer.on(HOST_STATE_CHANNELS.changed, (_e, state: HostState) =>
        cb(state),
      );
    },
  };
  contextBridge.exposeInMainWorld("hostState", hostState);
}
