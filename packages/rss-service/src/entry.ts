/**
 * RSS 数据服务组件 main（CJS，由 apps/desktop 宿主加载）。
 *
 * amoeba 数据服务（tauri-rss/dist/service/service.js，bun 运行时）是独立进程形态：
 * 单写者独占 SQLite、自带 HTTP/WS RPC + token 鉴权 + rev 事件协议 —— 桌面端本来就是
 * 它的客户端。本组件不重写、不分叉这套实现，只做生命周期宿主：
 *
 *   activate   → 定位 bun 与 service.js → spawn（独立 dataDir，--port 0）
 *              → 解析 stdout 的 `READY <port>` → provide("rss.endpoint")
 *   deactivate → POST /v1/shutdown（owner token）→ 等退出 → kill 兜底
 *
 * 抓取浏览器拆在独立组件 rss-browser（Electron CDP 代理）：本组件 manifest inject
 * `rss.browser`，activate 里 ctx.require 拿端口并以 `--browser-cdp` 传给服务进程。
 *
 * 运行前提：bun 在 PATH（或 AMOEBA_BUN 指定）；service.js 来自 AMOEBA_SERVICE_JS、
 * 组件包内（pack 时复制）或 tauri-rss 仓库的构建产物。探测失败 = 组件 error（可诊断）。
 */

import { spawn, type ChildProcess } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type { PageMainContext } from "@host/sdk";

/** 服务名：rss 域的数据服务端点（baseUrl + token + 生命周期） */
export const SERVICE_NAME = "rss.endpoint";

const READY_TIMEOUT_MS = 15_000;
const SHUTDOWN_TIMEOUT_MS = 5_000;

export interface RssEndpointInfo {
  baseUrl: string;
  port: number;
  /** owner token：持有者可调用全部 RPC（含清库/重置），组件内不外传给 renderer */
  token: string;
  dataDir: string;
  dbFile: string;
  pid: number;
}

export interface RssService {
  endpoint(): RssEndpointInfo;
  health(): Promise<{ ok: boolean; rev: number; uptimeMs: number; jobs: number }>;
  stop(): Promise<void>;
}

export interface RssServiceHandle {
  proc: ChildProcess;
  service: RssService;
  /** owner RPC（组件内部使用，token 不出组件边界） */
  call<T = unknown>(method: string, params?: Record<string, unknown>): Promise<T>;
}

/** rss-browser 组件提供的端点形状（只取用得到的字段，避免跨包类型依赖） */
interface BrowserEndpoint {
  endpoint(): { port: number };
  targetCount?(): number;
}

/* ---------- 运行时与产物定位 ---------- */

/** AMOEBA_BUN 显式指定；否则交给 PATH（spawn 解析），永远返回可用的启动命令 */
export function locateBun(): string {
  const env = process.env.AMOEBA_BUN;
  if (env !== undefined && env.length > 0) return env;
  return "bun";
}

/**
 * AMOEBA_SERVICE_JS（调试覆盖）→ 组件包内 service.js（pack 时由 runtime/ 本地构建）。
 * 不再回退外部仓库产物：数据服务源码已移植进本包（runtime/ + packages/amoeba/*）。
 */
export function locateServiceJs(componentRoot: string): string | undefined {
  const candidates = [
    process.env.AMOEBA_SERVICE_JS,
    join(componentRoot, "service.js"),
  ].filter((p): p is string => typeof p === "string" && p.length > 0);
  for (const p of candidates) {
    if (existsSync(p)) return p;
  }
  return undefined;
}

/* ---------- 启动（可独立测试） ---------- */

function waitReady(proc: ChildProcess, timeoutMs: number): Promise<number> {
  return new Promise((resolve, reject) => {
    let buffer = "";
    const timer = setTimeout(() => {
      cleanup();
      proc.kill();
      reject(new Error(`数据服务启动超时（${timeoutMs}ms 内未输出 READY）`));
    }, timeoutMs);
    const onData = (chunk: Buffer): void => {
      buffer += chunk.toString("utf8");
      const m = /(?:^|\n)READY (\d+)(?:\r?\n|$)/.exec(buffer);
      if (m) {
        cleanup();
        resolve(Number(m[1]));
      }
    };
    const onExit = (code: number | null): void => {
      cleanup();
      reject(
        new Error(`数据服务进程提前退出（code=${code}），详见组件日志中的 stderr`),
      );
    };
    const cleanup = (): void => {
      clearTimeout(timer);
      proc.stdout?.off("data", onData);
      proc.off("exit", onExit);
    };
    proc.stdout?.on("data", onData);
    proc.on("exit", onExit);
  });
}

function waitExit(proc: ChildProcess, timeoutMs: number): Promise<void> {
  return new Promise((resolve) => {
    if (proc.exitCode !== null || proc.signalCode !== null) return resolve();
    const timer = setTimeout(() => {
      proc.off("exit", onExit);
      proc.kill();
      resolve();
    }, timeoutMs);
    const onExit = (): void => {
      clearTimeout(timer);
      resolve();
    };
    proc.on("exit", onExit);
  });
}

export async function startRssService(opts: {
  bun: string;
  serviceJs: string;
  /** 服务独占的数据目录（rss.db / service.token / endpoint.json 都在里面） */
  dataDir: string;
  /** 宿主进程 pid：宿主崩溃时服务自杀（服务支持 --parent-pid 轮询） */
  parentPid: number;
  /** Electron CDP 代理端口：抓取浏览器（隐藏 BrowserWindow）由 rss-browser 组件提供；缺省 = 无浏览器 */
  browserCdpPort?: number;
  log(...args: unknown[]): void;
}): Promise<RssServiceHandle> {
  const tokenFile = join(opts.dataDir, "service.token");
  const proc = spawn(
    opts.bun,
    [
      opts.serviceJs,
      "--data-dir",
      opts.dataDir,
      "--token-file",
      tokenFile,
      "--port",
      "0",
      // 抓取浏览器 = Electron CDP 代理（替代 WebView2 sidecar）；未提供则关闭浏览器渠道
      ...(opts.browserCdpPort !== undefined
        ? ["--browser-cdp", String(opts.browserCdpPort)]
        : ["--no-browser"]),
      "--no-cookie-sync",
      "--parent-pid",
      String(opts.parentPid),
    ],
    { stdio: ["ignore", "pipe", "pipe"], windowsHide: true },
  );
  // 服务契约：stdout 只有 `READY <port>`，日志全在 stderr —— 转发进组件日志
  proc.stderr?.on("data", (chunk: Buffer) => {
    const text = chunk.toString("utf8").trimEnd();
    if (text) opts.log("[rss-service]", text);
  });

  const port = await waitReady(proc, READY_TIMEOUT_MS);
  const token = readFileSync(tokenFile, "utf8").trim();
  if (!token) throw new Error("service.token 为空");

  const baseUrl = `http://127.0.0.1:${port}`;
  const info: RssEndpointInfo = {
    baseUrl,
    port,
    token,
    dataDir: opts.dataDir,
    dbFile: join(opts.dataDir, "rss.db"),
    pid: proc.pid ?? -1,
  };
  const authed = (path: string, init?: RequestInit): Promise<Response> =>
    fetch(`${baseUrl}${path}`, {
      ...init,
      headers: { authorization: `Bearer ${token}`, ...init?.headers },
    });

  const call = async <T>(
    method: string,
    params?: Record<string, unknown>,
  ): Promise<T> => {
    const res = await authed("/v1/rpc", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ method, params: params ?? {} }),
      signal: AbortSignal.timeout(60_000),
    });
    const body = (await res.json()) as {
      ok: boolean;
      result?: T;
      error?: { kind?: string; message?: string };
    };
    if (!body.ok) {
      throw new Error(
        `[${body.error?.kind ?? `http-${res.status}`}] ${body.error?.message ?? "rpc failed"}`,
      );
    }
    return body.result as T;
  };

  let stopped = false;
  const service: RssService = {
    endpoint: () => ({ ...info }),
    async health() {
      const res = await authed("/v1/health");
      if (!res.ok) throw new Error(`/v1/health ${res.status}`);
      return (await res.json()) as {
        ok: boolean;
        rev: number;
        uptimeMs: number;
        jobs: number;
      };
    },
    async stop() {
      if (stopped) return;
      stopped = true;
      try {
        await authed("/v1/shutdown", { method: "POST" });
      } catch {
        // 服务可能已退出；kill 兜底在 waitExit
      }
      await waitExit(proc, SHUTDOWN_TIMEOUT_MS);
    },
  };
  return { proc, service, call };
}

/* ---------- 组件入口 ---------- */

let handle: RssServiceHandle | undefined;

export async function activate(ctx: PageMainContext): Promise<void> {
  const bun = locateBun();
  const serviceJs = locateServiceJs(ctx.pageRoot);
  if (serviceJs === undefined) {
    throw new Error(
      "找不到数据服务产物 service.js（设置 AMOEBA_SERVICE_JS，或 pack 时从 tauri-rss 复制）",
    );
  }
  // 抓取浏览器由 rss-browser 组件提供（manifest.inject 声明；拓扑保证先于本组件激活）
  const browser = ctx.require<BrowserEndpoint>("rss.browser");
  const browserPort = browser.endpoint().port;

  handle = await startRssService({
    bun,
    serviceJs,
    // 服务数据与组件 dataDir 分层：宿主分配 userData/page-data/rss-service/data
    dataDir: join(ctx.dataDir, "data"),
    parentPid: process.pid,
    browserCdpPort: browserPort,
    log: (...args) => ctx.log(...args),
  });
  ctx.provide(SERVICE_NAME, handle.service, {
    expose: ["endpoint", "health", "stop"],
  });
  // 扩展点：rss-channel-* 组件经此把渠道模块（channels.js）登记进数据服务。
  // 加载在执行插件代码，走 owner RPC（token 不出本组件）。
  const h = handle;
  ctx.provide(
    "rss.extend",
    {
      registerChannelModule: async (absPath: string) =>
        h.call<{
          registered: boolean;
          path: string;
          reason?: string;
          channels?: number;
        }>("channels.registerPlugin", { path: absPath }),
    },
    { expose: ["registerChannelModule"] },
  );
  ctx.log(
    `rss endpoint ready: ${handle.service.endpoint().baseUrl} (${serviceJs}); ` +
      `browser cdp proxy :${browserPort}`,
  );
}

export async function deactivate(): Promise<void> {
  const h = handle;
  handle = undefined;
  await h?.service.stop().catch(() => undefined);
  if (h && h.proc.exitCode === null) h.proc.kill();
}
