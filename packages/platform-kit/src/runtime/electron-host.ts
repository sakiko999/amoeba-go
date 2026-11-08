/**
 * Electron 宿主主进程的 appHost 适配层。
 *
 * 平台运行时代码（channel/client/playback/danmaku）统一经 `globalThis.appHost`
 * 访问宿主能力（HTTP / WS / 浏览器 / 时钟）。数据服务进程由 @amoeba/host 注入；
 * 现在平台组件直接跑在宿主主进程里，由本模块提供等价的最小实现：
 *   - http.request → Electron net.fetch（走系统证书）
 *   - ws.connect   → `ws` 包（自定义握手 header：douyin 需要 UA/Cookie/Origin）
 *   - browser      → 可选，由 browser.proxy 的 CDP 代理构造（见 extension.ts）
 */

import { net } from "electron";
import WebSocket from "ws";

interface HostHttpRequest {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string;
  responseType?: "text" | "json" | "arraybuffer";
  timeoutMs?: number;
  policy?: unknown;
}

interface HostHttpResponse {
  status: number;
  headers: Record<string, string>;
  body: unknown;
}

interface HostWsConnectOptions {
  url: string;
  headers?: Record<string, string>;
  timeoutMs?: number;
  onOpen?: (conn: HostWsConnection) => void;
  onMessage?: (data: Uint8Array, conn: HostWsConnection) => void;
  onClose?: (code: number, reason: string) => void;
}

interface HostWsConnection {
  readonly readyState: number;
  send(data: Uint8Array): void;
  close(): void;
}

/** 平台签名 blob（douyin ABogus / douyu CryptoJS / youtube n 参数）的执行面。 */
interface HostJsBackend {
  call(code: string, fn: string, args: (string | number)[]): unknown;
}

function nodeJsBackend(): HostJsBackend {
  // Electron 主进程无 CSP 限制；与 amoeba/host 的 nodeJsBackend 同实现。
  return {
    call(code: string, fn: string, args: (string | number)[]) {
      const factory = new Function(
        `${code}\n;return ${fn}(...arguments)`,
      ) as (...a: unknown[]) => unknown;
      return factory(...args);
    },
  };
}

function nodeWsBackend(): { connect(opts: HostWsConnectOptions): Promise<HostWsConnection> } {
  let nextId = 0;
  return {
    async connect(opts: HostWsConnectOptions): Promise<HostWsConnection> {
      const ws = new WebSocket(opts.url, { headers: opts.headers });
      let closed = false;
      const conn: HostWsConnection = {
        get readyState() {
          return ws.readyState;
        },
        send(data: Uint8Array): void {
          if (ws.readyState === WebSocket.OPEN) ws.send(data);
        },
        close(): void {
          ws.close();
        },
      };
      ws.on("open", () => opts.onOpen?.(conn));
      ws.on("message", (data) => {
        // ws 默认给 Buffer；统一转 Uint8Array（不要设 binaryType=arraybuffer）。
        const buf = data as Buffer;
        opts.onMessage?.(
          new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength),
          conn,
        );
      });
      ws.on("close", (code, reason) => {
        if (closed) return;
        closed = true;
        opts.onClose?.(code, reason.toString());
      });
      ws.on("error", (e) => {
        if (closed) return;
        closed = true;
        opts.onClose?.(1006, e.message);
      });
      void nextId;
      return conn;
    },
  };
}

async function hostHttpRequest(
  req: HostHttpRequest,
): Promise<HostHttpResponse> {
  const init: RequestInit = {
    method: req.method ?? "GET",
    ...(req.headers !== undefined ? { headers: req.headers } : {}),
    ...(req.body !== undefined ? { body: req.body } : {}),
    ...(req.timeoutMs !== undefined
      ? { signal: AbortSignal.timeout(req.timeoutMs) }
      : {}),
  };
  const res = await net.fetch(req.url, init);

  // set-cookie 必须回传：douyin 的 ttwid warmup 靠它抓新鲜 cookie，
  // bili 扫码登录也读它；恒空 headers 会让 cookie jar 永远失效（nodeBackend 同约定）。
  const rawSetCookie =
    res.headers.getSetCookie?.() ?? res.headers.get("set-cookie") ?? [];
  const setCookie = Array.isArray(rawSetCookie)
    ? rawSetCookie
    : rawSetCookie !== ""
      ? [rawSetCookie]
      : [];
  const headers: Record<string, string> =
    setCookie.length > 0 ? { "set-cookie": setCookie.join("\n") } : {};

  if (req.responseType === "arraybuffer") {
    return {
      status: res.status,
      headers,
      body: new Uint8Array(await res.arrayBuffer()),
    };
  }
  const text = await res.text();
  return {
    status: res.status,
    headers,
    body:
      req.responseType === "json"
        ? text.trim() === ""
          ? null
          : JSON.parse(text)
        : text,
  };
}

/**
 * 安装/补齐宿主主进程的 appHost。幂等：已存在的 http/now/ws/js 不覆盖；
 * browser **提供即覆盖** —— browser.proxy 每次激活端口都会变（临时端口），
 * 重载后依赖方重新 activate 时必须拿到新 backend，否则一直连旧端口。
 */
export function installPlatformAppHost(opts?: { browser?: unknown }): void {
  const g = globalThis as unknown as {
    appHost?: {
      http?: unknown;
      ws?: unknown;
      js?: unknown;
      now?: unknown;
      browser?: unknown;
    };
  };
  if (g.appHost === undefined) g.appHost = {};
  const host = g.appHost;
  host.now ??= () => Date.now();
  host.http ??= { request: hostHttpRequest };
  host.ws ??= nodeWsBackend();
  host.js ??= nodeJsBackend();
  if (opts?.browser !== undefined) {
    host.browser = opts.browser;
  }
}
