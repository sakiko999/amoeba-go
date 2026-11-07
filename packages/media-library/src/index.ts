/**
 * @tabs/media-library 公共入口。
 * - 独立形态（media-manager）：createMediaService + bindMediaIpc 组装到 ipcMain
 * - 宿主服务组件（scripts/pack.ts 构建的 serviceEntry）：ctx.provide("media.library")
 *   + media.providers 扩展点（元数据渠道由 media-provider-* 扩展组件注册）
 * - 扩展组件（media-provider-*）：ProviderRegistry / MetadataProvider 与
 *   provider 基础工具（HTTP/缓存/错误）从这里导入
 * - renderer / preload：只用类型（MediaApi / LibraryItem / ShowDetail 等）
 */

export { bindMediaIpc } from "./main/ipcBinding";
export { ProviderRegistry } from "./main/providers";
export type { MetadataProvider, MetadataProviderSource } from "./main/providers";
export {
  NotFoundError,
  getJsonCached,
  postJsonCached,
  fetchBinary,
  ProviderError,
  USER_AGENT,
} from "./main/provider/http";
export { cacheGet, cacheKey, cacheSet } from "./main/cache";
export { scrapeApply, scrapePreview, searchMetadata } from "./main/scrape";
export { createMediaService } from "./main/service";
export type { MediaService, MediaServiceOptions } from "./main/service";
export { SERVICE_NAME } from "./main/serviceEntry";
export * from "./shared/model";
export * from "./shared/types";
