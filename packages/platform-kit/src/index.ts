/**
 * platform 插件（扩展组件）的入口工厂。所有平台插件把能力注册进
 * api.gateway（统一平台注册中心，见 packages/api-gateway）：
 *
 * - rss 平台（defineRssPlatformExtension）：组件 main 在宿主主进程，
 *   ① 经 rss.extend 把本包 channels.js（Bun 服务进程侧 bundle）交给数据服务
 *     （渠道与 playback 的执行体在那里）；② 读 pack 生成的 api.json，把平台
 *   渠道的静态描述（key/name/kind/参数表）注册成 `rss.<平台>` API。
 * - 媒体元数据平台（defineMediaPlatformExtension）：provider 对象直接包成
 *   `media.<source>` API（search / fetchMetadata / fetchEpisodes）。
 *
 * 卸载语义（v1）：注册活在消费方（网关/数据服务）内，组件停用/卸载后不撤销，
 * 直到基础能力组件 reload。
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { PageMainContext } from "@host/sdk";

/** rss-service 组件提供的扩展点形状（只取用得到的字段） */
interface RssExtend {
  registerChannelModule(absPath: string): Promise<{
    registered: boolean;
    path: string;
    reason?: string;
    channels?: number;
  }>;
}

/** api-gateway 组件提供的注册面 */
interface ApiGateway {
  register(reg: {
    namespace: string;
    label: string;
    ops: Record<string, { description?: string; invoke: (params: unknown) => Promise<unknown> | unknown }>;
  }): void;
}

/** pack 生成的渠道静态描述（api.json） */
interface ChannelMeta {
  key: string;
  name: string;
  kind: string;
  passive?: boolean;
  sourceInfoTpl?: Array<{ key: string; label: string; required?: boolean; placeholder?: string }>;
}

export interface PlatformExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

/** rss 平台插件：登记 channels.js（执行体）+ 渠道元数据 API（rss.<平台>）。 */
export function defineRssPlatformExtension(): PlatformExtensionModule {
  return {
    async activate(ctx) {
      const ext = ctx.require<RssExtend>("rss.extend");
      const modulePath = join(ctx.pageRoot, "channels.js");
      const result = await ext.registerChannelModule(modulePath);
      ctx.log(
        `channels plugin ${result.registered ? "registered" : `skipped (${result.reason ?? "loaded"})`}: ` +
          modulePath,
      );

      // pack 阶段从 channels bundle 提取的静态描述；缺失只意味着老产物，跳过
      const apiJsonPath = join(ctx.pageRoot, "api.json");
      let meta: { namespace: string; label: string; channels: ChannelMeta[] } | undefined;
      try {
        meta = JSON.parse(readFileSync(apiJsonPath, "utf8")) as typeof meta;
      } catch {
        /* 无 api.json：跳过网关注册 */
      }
      if (meta !== undefined && Array.isArray(meta.channels) && meta.channels.length > 0) {
        const gw = ctx.require<ApiGateway>("api.gateway");
        gw.register({
          namespace: meta.namespace,
          label: meta.label,
          ops: {
            channels: {
              description: "平台渠道清单（静态描述；抓取执行在 rss 数据服务进程）",
              invoke: () => meta!.channels,
            },
          },
        });
      }
    },
    deactivate() {
      // 渠道注册在数据服务进程内；停用不撤销（v1），服务 reload 后生效
    },
  };
}

/** 媒体平台 provider 的最小结构（与 @tabs/media-library 的 MetadataProvider 兼容）。 */
interface MetadataProviderLike {
  search(keyword: string, mediaType: string): Promise<unknown[]>;
  fetchMetadata(id: string, mediaType: string): Promise<unknown>;
  fetchEpisodes(meta: unknown, id: string): Promise<unknown[]>;
}

export interface MediaPlatformSpec {
  /** 渠道身份（ScrapeSource，如 "tmdb"）；API 命名空间 = media.<source> */
  readonly source: string;
  /** 人读标签（如 "TMDB"） */
  readonly label: string;
  /** provider 工厂（独立形态经包根导出复用） */
  readonly createProvider: () => MetadataProviderLike;
}

/** 媒体元数据平台插件：provider 包成 media.<source> API 注册进网关。 */
export function defineMediaPlatformExtension(spec: MediaPlatformSpec): PlatformExtensionModule {
  return {
    async activate(ctx) {
      const gw = ctx.require<ApiGateway>("api.gateway");
      const provider = spec.createProvider();
      gw.register({
        namespace: `media.${spec.source}`,
        label: `媒体元数据 · ${spec.label}`,
        ops: {
          search: {
            description: "按关键词搜索条目",
            invoke: (params) => {
              const p = params as { keyword?: string; mediaType?: string };
              return provider.search(String(p.keyword ?? ""), String(p.mediaType ?? "tv"));
            },
          },
          fetchMetadata: {
            description: "按平台条目 id 取完整元数据",
            invoke: (params) => {
              const p = params as { id?: string; mediaType?: string };
              return provider.fetchMetadata(String(p.id ?? ""), String(p.mediaType ?? "tv"));
            },
          },
          fetchEpisodes: {
            description: "取剧集每集元数据（不支持的平台返回空数组）",
            invoke: (params) => {
              const p = params as { meta?: unknown; id?: string };
              return provider.fetchEpisodes(p.meta, String(p.id ?? ""));
            },
          },
        },
      });
      ctx.log(`media api registered: media.${spec.source}`);
    },
    deactivate() {
      // 注册表在网关内；停用不撤销（v1），随网关 reload 重建
    },
  };
}
