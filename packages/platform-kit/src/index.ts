/**
 * platform 插件（扩展组件）的入口工厂（媒体元数据渠道）。
 *
 * - 媒体元数据平台（defineMediaPlatformExtension）：provider 对象直接包成
 *   `<source>` API（search / fetchMetadata / fetchEpisodes，tags: media.metadata）。
 * - 数据渠道平台（直播/社媒/视频源类）走 `runtime/extension` 的
 *   definePlatformExtension（channels / fetch / play / danmaku）。
 *
 * 卸载语义（v1）：注册活在消费方（网关）内，组件停用/卸载后不撤销，
 * 直到基础能力组件 reload。
 */

import type { PageMainContext } from "@host/sdk";
import type {
  MediaMetadata,
  MediaType,
  MetadataProvider,
} from "@tabs/platform-contracts";

/** api-gateway 组件提供的注册面 */
interface ApiGateway {
  register(reg: {
    namespace: string;
    label: string;
    tags?: string[];
    ops: Record<string, { description?: string; invoke: (params: unknown) => Promise<unknown> | unknown }>;
  }): void;
}

export interface PlatformExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

export interface MediaPlatformSpec {
  /** 渠道身份（ScrapeSource，如 "tmdb"）；API 命名空间 = 平台 id 本身 */
  readonly source: string;
  /** 人读标签（如 "TMDB"） */
  readonly label: string;
  /** provider 工厂（独立形态经包根导出复用）；契约见 @tabs/platform-contracts */
  readonly createProvider: () => MetadataProvider;
}

/** 媒体元数据平台插件：provider 包成 <source> API 注册进网关（平台即命名空间）。 */
export function defineMediaPlatformExtension(spec: MediaPlatformSpec): PlatformExtensionModule {
  return {
    async activate(ctx) {
      const gw = ctx.require<ApiGateway>("api.gateway");
      const provider = spec.createProvider();
      gw.register({
        namespace: spec.source,
        label: spec.label,
        tags: ["media.metadata"],
        ops: {
          search: {
            description: "按关键词搜索条目",
            invoke: (params) => {
              const p = params as { keyword?: string; mediaType?: MediaType };
              return provider.search(String(p.keyword ?? ""), p.mediaType ?? "tv");
            },
          },
          fetchMetadata: {
            description: "按平台条目 id 取完整元数据",
            invoke: (params) => {
              const p = params as { id?: string; mediaType?: MediaType };
              return provider.fetchMetadata(String(p.id ?? ""), p.mediaType ?? "tv");
            },
          },
          fetchEpisodes: {
            description: "取剧集每集元数据（不支持的平台返回空数组）",
            invoke: (params) => {
              const p = params as { meta?: unknown; id?: string };
              return provider.fetchEpisodes(p.meta as MediaMetadata, String(p.id ?? ""));
            },
          },
        },
      });
      ctx.log(`media api registered: ${spec.source}`);
    },
    deactivate() {
      // 注册表在网关内；停用不撤销（v1），随网关 reload 重建
    },
  };
}
