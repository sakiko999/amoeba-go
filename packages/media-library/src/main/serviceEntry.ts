/**
 * 服务组件 main 扩展（CJS，由 apps/desktop 宿主加载）。
 * 把媒体能力注册为宿主服务 media.library：页面组件（如 media-manager）经
 * manifest inject 声明后在 activate 里 ctx.require 消费；服务提供方自身
 * 无 entry/preload，不建 tab，由宿主按依赖拓扑先于页面激活。
 * dataDir 为宿主分配的 userData/page-data/media-library（sqlite 缓存落这里）。
 */

import type { BrowserWindow } from "electron";
import type { PageMainContext } from "@host/sdk";
import { ProviderRegistry } from "./providers";
import { createMediaService } from "./service";
import type { MediaService } from "./service";

/** 服务名：media 域的库能力（扫描/详情/刮削/NFO/整理/缓存） */
export const SERVICE_NAME = "media.library";

let service: MediaService | undefined;

export function activate(ctx: PageMainContext): void {
  // 元数据渠道扩展点：media-provider-* 组件把 provider 注册进同一实例
  const providers = new ProviderRegistry();
  service = createMediaService({
    getWindow: () => ctx.getWindow() as BrowserWindow | undefined,
    dataDir: ctx.dataDir,
    providers,
  });
  ctx.provide(SERVICE_NAME, service, {
    expose: [
      "pickLibrary",
      "scanLibrary",
      "showDetail",
      "imageToDataUrl",
      "reveal",
      "searchMetadata",
      "scrapePreview",
      "scrapeApply",
      "readNfo",
      "writeNfo",
      "organizePlan",
      "organizeApply",
      "cacheClear",
    ],
  });
  ctx.provide(
    "media.providers",
    {
      register: (provider: unknown) => providers.register(provider as never),
      listSources: () => providers.listSources(),
    },
    { expose: ["register", "listSources"] },
  );
  ctx.log("media.library service registered (media.providers open)");
}

export function deactivate(): void {
  service?.dispose();
  service = undefined;
}
