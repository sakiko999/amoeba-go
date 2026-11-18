/**
 * 页面组件 main 扩展（CJS，由 apps/desktop 宿主加载）。
 * 后端能力已拆到服务组件 media-library（ctx.require("media.library")）；
 * 这里只做两件事：取服务 + 把渲染侧 IPC 绑定到服务方法（channel 契约不变，
 * renderer / preload 零改动）。不建窗口、不注册协议、不接管 app 生命周期。
 */

import type { PageMainContext } from "@host/sdk";
import { bindMediaIpc } from "@tabs/media-library";
import type { MediaService } from "@tabs/media-library";

let unbind: (() => void) | undefined;

export function activate(ctx: PageMainContext): void {
  // manifest.inject 声明了 media.library；宿主按拓扑先激活服务组件，
  // 未就绪时 require 抛错、本页面进入 error 态（错误信息带服务名）
  const service = ctx.require<MediaService>("media.library");
  unbind = bindMediaIpc(ctx.ipc, service, `page://${ctx.manifest.id}/`);
  ctx.log("media page activated (backend: media.library service)");
}

export function deactivate(): void {
  unbind?.();
  unbind = undefined;
}
