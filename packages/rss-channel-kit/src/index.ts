/**
 * 渠道扩展组件的入口工厂。
 *
 * 扩展组件（kind=service）只做一件事：把组件包内的 `channels.js`
 * （由 scripts/pack.ts 构建，导出 register()）登记进数据服务。
 * 渠道代码在 Bun 服务进程里执行；组件 main 跑在宿主主进程，二者经
 * rss.extend（rss-service 组件提供）的 registerChannelModule 交接路径。
 *
 * 卸载语义（v1）：渠道注册活在数据服务进程内，组件停用/卸载后渠道保留，
 * 直到数据服务重启（rss-service 组件 reload）才消失。文档写明。
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

export interface ChannelExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

export function defineChannelExtension(): ChannelExtensionModule {
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
