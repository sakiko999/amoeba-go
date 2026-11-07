/**
 * @tabs/media-library 公共入口。
 * - 独立形态（media-manager）：createMediaService + bindMediaIpc 组装到 ipcMain
 * - 宿主服务组件（scripts/pack.ts 构建的 serviceEntry）：ctx.provide("media.library")
 * - renderer / preload：只用类型（MediaApi / LibraryItem / ShowDetail 等）
 */

export { bindMediaIpc } from "./main/ipcBinding";
export { createMediaService } from "./main/service";
export type { MediaService, MediaServiceOptions } from "./main/service";
export { SERVICE_NAME } from "./main/serviceEntry";
export * from "./shared/model";
export * from "./shared/types";
