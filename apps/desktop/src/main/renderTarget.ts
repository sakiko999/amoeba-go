/**
 * renderer 页面寻址：dev 用 electron-vite 注入的 ELECTRON_RENDERER_URL（HMR），
 * 生产用 app:// 自定义协议（main 里 protocol.handle 映射到 out/renderer）——
 * ESM 模块脚本需要 http 型 origin，file:// 下会被 CORS 拦截。
 * 路径约定：页面相对路径 = 源文件相对仓库根（renderer root 即仓库根），产物同名同址。
 */

const DEV_URL = process.env.ELECTRON_RENDERER_URL;

function pageUrl(relPath: string): string {
  return DEV_URL ? `${DEV_URL}/${relPath}` : `app://r/${relPath}`;
}

export function hostUrl(): string {
  return pageUrl("apps/desktop/src/ui/index.html");
}
