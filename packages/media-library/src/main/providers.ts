/**
 * 元数据渠道扩展点：接口 + 注册表。
 *
 * 基础服务（media.library）不内置任何平台渠道；TMDB / 豆瓣 / Bangumi 等由
 * platform-* 扩展组件提供：
 *   - 宿主形态：平台组件把 API 注册进 api.gateway（统一平台注册中心），
 *     media-library 组件 inject api.gateway 并按 media.metadata 标签惰性解析
 *   - 独立形态（media-manager app）：backend.ts 直接注册同一批 provider 包
 *
 * 写盘/安全闸门（组织整理、NFO 写回）仍留在 media-library，provider 只取数。
 */

import type {
  EpisodeMetadata,
  MediaMetadata,
  MediaType,
  SearchResult,
} from "../shared/model";
import type { ScrapeSource } from "../shared/types";

export interface MetadataProvider {
  /** 渠道身份（与 ScrapeSource 一致，如 "tmdb"） */
  readonly source: ScrapeSource;
  search(keyword: string, mediaType: MediaType): Promise<SearchResult[]>;
  fetchMetadata(id: string, mediaType: MediaType): Promise<MediaMetadata>;
  /** tv 每集元数据；渠道不支持返回 [] */
  fetchEpisodes(meta: MediaMetadata, id: string): Promise<EpisodeMetadata[]>;
}

/** 渠道解析面：静态注册表与「经 api.gateway 动态解析」都实现它。 */
export interface MetadataProviderSource {
  register(provider: MetadataProvider): void;
  get(source: ScrapeSource): MetadataProvider;
  has(source: ScrapeSource): boolean;
  listSources(): ScrapeSource[];
}

export class ProviderRegistry implements MetadataProviderSource {
  private readonly providers = new Map<ScrapeSource, MetadataProvider>();

  /** 注册（同 source 后者覆盖，与服务协议的幂等覆盖语义一致） */
  register(provider: MetadataProvider): void {
    if (typeof provider?.source !== "string" || provider.source.length === 0) {
      throw new Error("MetadataProvider.source 不能为空");
    }
    this.providers.set(provider.source, provider);
  }

  /** 取渠道；未安装扩展时不静默降级，直接报可诊断错误 */
  get(source: ScrapeSource): MetadataProvider {
    const provider = this.providers.get(source);
    if (provider === undefined) {
      throw new Error(
        `未安装 ${source} 元数据渠道组件（platform-${source}）`,
      );
    }
    return provider;
  }

  has(source: ScrapeSource): boolean {
    return this.providers.has(source);
  }

  listSources(): ScrapeSource[] {
    return [...this.providers.keys()];
  }
}
