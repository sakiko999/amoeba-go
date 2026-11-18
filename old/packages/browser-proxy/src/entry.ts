/**
 * 抓取浏览器组件 main（CJS）：提供 Electron 版的无头浏览器能力。
 *
 * 数据服务（rss-service 组件）以 `--browser-cdp <port>` 接入本组件暴露的 CDP 代理，
 * 替代 tauri-rss 原 WebView2 sidecar（amoeba-browser.exe）。浏览器本体是宿主进程里
 * 的隐藏 BrowserWindow（session 分区 persist:rss-crawler），不额外起进程。
 *
 * provides browser.proxy:
 *   - endpoint(): { port }   —— CDP 代理端口（HTTP /json/* + WS /devtools/*）
 *   - targetCount(): number  —— 当前托管的隐藏窗口数（诊断用）
 *
 * activate 内会做一次端到端自检（browser WS → createTarget → page WS →
 * Network/Page/Runtime 命令 → closeTarget），失败即组件 error（可诊断）。
 */

import type { PageMainContext } from "@host/sdk";
import { startCdpProxy, type CdpProxy } from "./proxy";

export const SERVICE_NAME = "browser.proxy";

let proxy: CdpProxy | undefined;

export async function activate(ctx: PageMainContext): Promise<void> {
  proxy = await startCdpProxy({
    log: (...args) => ctx.log(...args),
    blockImages: true,
  });
  const ua = await proxy.selfTest();
  const p = proxy;
  ctx.provide(
    SERVICE_NAME,
    {
      endpoint: () => ({ port: p.port }),
      targetCount: () => p.targetCount(),
    },
    { expose: ["endpoint", "targetCount"] },
  );
  ctx.log(`browser.proxy ready: cdp proxy :${p.port} (UA: ${ua})`);
}

export async function deactivate(): Promise<void> {
  const p = proxy;
  proxy = undefined;
  await p?.close().catch(() => undefined);
}
