/**
 * 元数据渠道解析面：接口 + 注册表。
 *
 * 平台统一契约（MetadataProvider 与数据模型）在 @tabs/platform-contracts；
 * 平台插件（platform-media-*）与 media-library 都只依赖契约，互不依赖：
 *   - 宿主形态：平台组件把 API 注册进 api.gateway（统一平台注册中心），
 *     media-library 组件 inject api.gateway 并按 media.metadata 标签惰性解析
 *   - 独立形态（media-manager app）：backend.ts 直接注册同一批 provider 包
 *
 * 写盘/安全闸门（组织整理、NFO 写回）仍留在 media-library，provider 只取数。
 */

import type { MetadataProvider } from "@tabs/platform-contracts";

import type { ScrapeSource } from "../shared/types";

export type { MetadataProvider } from "@tabs/platform-contracts";

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
    this.providers.set(provider.source as ScrapeSource, provider);
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
