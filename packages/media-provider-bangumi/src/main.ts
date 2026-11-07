/** Bangumi 元数据渠道扩展组件：向 media.providers 注册 provider（同进程直接传对象）。 */

import type { PageMainContext } from "@host/sdk";
import type { MetadataProvider } from "@tabs/media-library";
import { createBangumiProvider } from "./provider";

export { createBangumiProvider } from "./provider";

interface ProvidersExtend {
  register(provider: MetadataProvider): void;
}

export function activate(ctx: PageMainContext): void {
  const providers = ctx.require<ProvidersExtend>("media.providers");
  providers.register(createBangumiProvider());
  ctx.log("bangumi metadata provider registered");
}

export function deactivate(): void {
  // 注册表在 media-library 组件内；停用不撤销（v1，随基础组件 reload 重建）
}