/**
 * 服务组件 main 扩展（CJS，由 apps/desktop 宿主加载）。
 * 把媒体能力注册为宿主服务 media.library：页面组件（如 media-manager）经
 * manifest inject 声明后在 activate 里 ctx.require 消费；服务提供方自身
 * 无 entry/preload，不建 tab，由宿主按依赖拓扑先于页面激活。
 * dataDir 为宿主分配的 userData/page-data/media-library（sqlite 缓存落这里）。
 *
 * 元数据渠道经 api.gateway（统一平台注册中心）惰性解析：platform-media-* 组件
 * 把 provider 包成 media.<source> API 注册；本组件刮削时按命名空间动态取用，
 * 因此不依赖平台组件的激活顺序。独立形态（media-manager）不走网关，
 * backend.ts 直接静态注册同一批 provider 包。
 */

import type { BrowserWindow } from "electron";
import type { PageMainContext } from "@host/sdk";
import { createMediaService } from "./service";
import type { MediaService } from "./service";
import type {
  MetadataProvider,
  MetadataProviderSource,
} from "./providers";
import type {
  EpisodeMetadata,
  MediaMetadata,
  MediaType,
  SearchResult,
} from "../shared/model";
import type { ScrapeSource } from "../shared/types";

/** 服务名：media 域的库能力（扫描/详情/刮削/NFO/整理/缓存） */
export const SERVICE_NAME = "media.library";

/** api-gateway 组件提供的调用面（只取用得到的字段） */
interface ApiGateway {
  call(op: string, params?: unknown): Promise<unknown>;
  list(): Array<{ namespace: string; label: string }>;
}

let service: MediaService | undefined;

/** 网关侧的 provider 适配：把 media.<source> API 包回 MetadataProvider。 */
function providerFromGateway(
  gateway: ApiGateway,
  source: string,
): MetadataProvider {
  const ns = `media.${source}`;
  return {
    source: source as ScrapeSource,
    search: (keyword, mediaType) =>
      gateway.call(`${ns}.search`, { keyword, mediaType }) as Promise<SearchResult[]>,
    fetchMetadata: (id, mediaType) =>
      gateway.call(`${ns}.fetchMetadata`, { id, mediaType }) as Promise<MediaMetadata>,
    fetchEpisodes: (meta, id) =>
      gateway.call(`${ns}.fetchEpisodes`, { meta, id }) as Promise<EpisodeMetadata[]>,
  };
}

/** 惰性渠道解析面：list/get 都实时查网关，平台组件晚注册也能被发现。 */
function gatewayProviderSource(gateway: ApiGateway): MetadataProviderSource {
  const sources = (): ScrapeSource[] =>
    gateway
      .list()
      .filter((api) => api.namespace.startsWith("media."))
      .map((api) => api.namespace.slice("media.".length) as ScrapeSource);
  return {
    // 网关模式下不支持外部注册（注册发生在 platform-media 组件里）
    register: () => {
      throw new Error("宿主形态的渠道注册走 api.gateway（platform-media 组件），不支持直接 register");
    },
    get(source) {
      if (!sources().includes(source)) {
        throw new Error(
          `未安装 ${source} 元数据渠道组件（platform-media-${source}，经 api.gateway）`,
        );
      }
      return providerFromGateway(gateway, source);
    },
    has(source) {
      return sources().includes(source);
    },
    listSources: sources,
  };
}

export function activate(ctx: PageMainContext): void {
  const gateway = ctx.require<ApiGateway>("api.gateway");
  service = createMediaService({
    getWindow: () => ctx.getWindow() as BrowserWindow | undefined,
    dataDir: ctx.dataDir,
    providers: gatewayProviderSource(gateway),
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
  ctx.log("media.library service registered (metadata channels resolve lazily via api.gateway)");
}

export function deactivate(): void {
  service?.dispose();
  service = undefined;
}
