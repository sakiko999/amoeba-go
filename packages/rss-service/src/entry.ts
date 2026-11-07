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
 * 消费方（未来的 RSS 页面组件）manifest inject rss.endpoint 后拿 { baseUrl, port,
 * token }，用 @amoeba/service-client 的 RemoteDataLayer 直连 —— RPC/事件/鉴权全部
 * 复用服务自身协议，凭证不过组件边界。
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
}

/* ---------- 运行时与产物定位 ---------- */

/** AMOEBA_BUN 显式指定；否则交给 PATH（spawn 解析），永远返回可用的启动命令 */
export function locateBun(): string {
  const env = process.env.AMOEBA_BUN;
  if (env !== undefined && env.length > 0) return env;
  return "bun";
}

/** AMOEBA_SERVICE_JS → 组件包内 service.js（pack 复制）→ tauri-rss 构建产物（开发机兜底） */
export function locateServiceJs(componentRoot: string): string | undefined {
  const candidates = [
    process.env.AMOEBA_SERVICE_JS,
    join(componentRoot, "service.js"),
    "X:\\tauri-rss\\dist\\service\\service.js",
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
      // v1 不注入抓取浏览器：浏览器渠道（xhs/douyin 等）不可用，RSS/API 渠道不受影响
      "--no-browser",
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
  return { proc, service };
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
  handle = await startRssService({
    bun,
    serviceJs,
    // 服务数据与组件 dataDir 分层：宿主分配 userData/page-data/rss-service/data
    dataDir: join(ctx.dataDir, "data"),
    parentPid: process.pid,
    log: (...args) => ctx.log(...args),
  });
  ctx.provide(SERVICE_NAME, handle.service, {
    expose: ["endpoint", "health", "stop"],
  });
  ctx.log(
    `rss endpoint ready: ${handle.service.endpoint().baseUrl} (${serviceJs})`,
  );
}

export async function deactivate(): Promise<void> {
  const h = handle;
  handle = undefined;
  await h?.service.stop().catch(() => undefined);
  if (h && h.proc.exitCode === null) h.proc.kill();
}
