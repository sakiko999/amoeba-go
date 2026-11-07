/**
 * platform 插件（扩展组件）的入口工厂。两种能力共用同一套组件形态：
 *
 * - rss 渠道（defineRssPlatformExtension）：组件 main 在宿主主进程，
 *   把本包的 channels.js（Bun 服务进程侧 bundle）经 rss.extend 的
 *   registerChannelModule 交接给数据服务；渠道实现在数据服务进程执行。
 * - 媒体元数据渠道（defineMediaPlatformExtension）：组件 main 在宿主主进程，
 *   直接把 provider 对象注册进 media.providers（同进程，扩展点由 media-library
 *   组件提供）。
 *
 * 卸载语义（v1）：注册活在消费方进程内，组件停用/卸载后不撤销，直到
 * 基础能力组件 reload。
 */

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

/** media-library 组件提供的扩展点形状 */
interface MediaProvidersExtend {
  register(provider: unknown): void;
}

export interface PlatformExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

/** rss 渠道 platform 插件：登记组件包内的 channels.js。 */
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
    },
    deactivate() {
      // 渠道注册在数据服务进程内；停用不撤销（v1），服务 reload 后生效
    },
  };
}

/** 媒体元数据 platform 插件：把工厂创建出的 provider 注册进 media.providers。 */
export function defineMediaPlatformExtension(
  createProvider: () => unknown,
): PlatformExtensionModule {
  return {
    async activate(ctx) {
      const providers = ctx.require<MediaProvidersExtend>("media.providers");
      providers.register(createProvider());
      ctx.log("metadata provider registered");
    },
    deactivate() {
      // 注册表在 media-library 组件内；停用不撤销（v1），随基础组件 reload 重建
    },
  };
}
