/**
 * 媒体后端 IPC（独立应用入口 src/main/index.ts 使用）：
 * createMediaService + bindMediaIpc 到 ipcMain 的组合，能力核心在
 * @tabs/media-library（主仓库 packages/media-library）。
 * 这里不创建窗口、不注册协议、不监听 app 生命周期 —— 纯能力注册，可 install/dispose。
 * 宿主页面扩展形态走 pageEntry.ts（服务来自宿主服务组件），不经过本文件。
 */

import { ipcMain } from "electron";
import type { BrowserWindow } from "electron";
import type { PageIpc } from "@host/sdk";
import {
  bindMediaIpc,
  createMediaService,
  ProviderRegistry,
} from "@tabs/media-library";
import type { MediaService } from "@tabs/media-library";
import { createBangumiProvider } from "@tabs/platform-bangumi/provider";
import { createDoubanProvider } from "@tabs/platform-douban/provider";
import { createTmdbProvider } from "@tabs/platform-tmdb/provider";

export interface MediaBackendOptions {
  getWindow(): BrowserWindow | undefined;
  /** 默认直接用 ipcMain；页面模式传宿主注入的受控注册器（卸载时自动清理） */
  ipc?: PageIpc;
  /** 缓存目录；页面模式用宿主提供的 page-data 目录，独立应用默认 userData */
  dataDir?: string;
  /** 设置后只接受该 URL 前缀的 sender（页面模式传 page://<id>/） */
  allowedOrigin?: string;
}

export interface MediaBackend {
  dispose(): void;
}

export function registerMediaBackend(opts: MediaBackendOptions): MediaBackend {
  // 独立形态静态装配元数据渠道（宿主形态由 platform-* 组件动态注册）
  const providers = new ProviderRegistry();
  providers.register(createTmdbProvider());
  providers.register(createDoubanProvider());
  providers.register(createBangumiProvider());
  const service: MediaService = createMediaService({
    getWindow: opts.getWindow,
    dataDir: opts.dataDir,
    providers,
  });
  const unbind = bindMediaIpc(opts.ipc ?? ipcMain, service, opts.allowedOrigin);
  return {
    dispose(): void {
      unbind();
      service.dispose();
    },
  };
}
