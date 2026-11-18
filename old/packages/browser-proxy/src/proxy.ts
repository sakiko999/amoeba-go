/**
 * Electron CDP 代理：把 amoeba crawler（tauri-rss 数据服务）的浏览器载具
 * 从 WebView2 sidecar 换成 Electron 自带 Chromium —— 对 host 侧呈现标准
 * Chromium CDP 端点，内部每个 target 是一个隐藏 BrowserWindow，
 * page 连接转发到 `webContents.debugger.sendCommand`。
 *
 * host（rss-service 组件 spawn 的 bun 数据服务）以 `--browser-cdp <port>` 接入，
 * 复用其 @amoeba/host 的 CdpBrowserBackend 全部语义（lane/超时/资源拦截/cookie）：
 *   - `/json/version|list|close/:id|activate/:id`（HTTP，host 的 target 管理）
 *   - browser WS：Target.createTarget → 新建隐藏窗口；Target.closeTarget/getTargets/…
 *   - page WS：任意 CDP 命令 → debugger.sendCommand；debugger 事件 → 文本帧广播
 *
 * 与 WebView2 sidecar 的差异（有意的）：
 *   - UA 洗成纯 Chromium（Electron 标识会直接触发强反爬平台风控）；
 *   - 隐藏窗口以 backgroundThrottling:false 全速渲染（没有 SW_HIDE 停渲染模式），
 *     滚动驱动需要的 rAF/IntersectionObserver 天然可用；
 *   - profile = Electron session 分区 `persist:rss-crawler`（登录态存 userData）。
 * ⚠️ 指纹不只 UA：canvas/WebGL 等仍与 WebView2 不同，强反爬平台需真机验证。
 * ⚠️ 端口只绑 127.0.0.1 且无鉴权 —— 与 sidecar CDP 端口同一信任模型（本机）。
 */

import { createServer, type IncomingMessage, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { randomUUID } from "node:crypto";
import { BrowserWindow, session } from "electron";
import { WebSocket, WebSocketServer, type RawData } from "ws";

const PARTITION = "persist:rss-crawler";
const SELF_TEST_TIMEOUT_MS = 8_000;

export interface CdpProxyOptions {
  log?(...args: unknown[]): void;
  /** 关闭图片加载（与 sidecar --no-images 对齐；登录二维码走属性/canvas，不依赖图片渲染）。 */
  blockImages?: boolean;
}

export interface CdpProxy {
  readonly port: number;
  targetCount(): number;
  /** 关掉全部托管窗口（host 空闲回收语义；不动 Electron 本体）。 */
  closeTargets(): Promise<void>;
  /** 清 rss-crawler 分区的全部存储（profile 重置）。 */
  resetProfile(): Promise<void>;
  /** 端到端自检：browser WS → createTarget → page WS → evaluate → closeTarget。 */
  selfTest(): Promise<string>;
  /** 组件停用：关窗口 + 停服务端。 */
  close(): Promise<void>;
}

interface ManagedTarget {
  id: string;
  win: BrowserWindow;
  attached: boolean;
}

function decodeText(data: RawData): string {
  if (typeof data === "string") return data;
  if (Buffer.isBuffer(data)) return data.toString("utf8");
  if (Array.isArray(data)) return Buffer.concat(data).toString("utf8");
  return Buffer.from(data).toString("utf8");
}

/** 当前 Chromium 版本构造纯 Chrome UA（去掉 Electron 段）。 */
function chromiumUserAgent(): string {
  const major = process.versions.chrome.split(".")[0] ?? "141";
  return (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
    `(KHTML, like Gecko) Chrome/${major}.0.0.0 Safari/537.36`
  );
}

export async function startCdpProxy(opts: CdpProxyOptions = {}): Promise<CdpProxy> {
  const log = opts.log ?? (() => undefined);
  const targets = new Map<string, ManagedTarget>();
  const pageSockets = new Map<string, Set<WebSocket>>();
  const ses = session.fromPartition(PARTITION);
  ses.setUserAgent(chromiumUserAgent());

  if (opts.blockImages === true) {
    ses.webRequest.onBeforeRequest({ urls: ["<all_urls>"] }, (details, callback) => {
      callback({ cancel: details.resourceType === "image" });
    });
  }

  /** debugger 事件 → 该 target 的所有 page 连接（只注册一次/每 wc）。 */
  function attachDebugger(target: ManagedTarget): void {
    if (target.attached) return;
    const wc = target.win.webContents;
    try {
      wc.debugger.attach("1.3");
    } catch {
      // 已附着（例如窗口复用时被上一轮 attach）——事件监听保持幂等
    }
    target.attached = true;
    wc.debugger.on("message", (_event, method, params) => {
      const sockets = pageSockets.get(target.id);
      if (!sockets || sockets.size === 0) return;
      const frame = JSON.stringify({ method, params });
      for (const ws of sockets) {
        if (ws.readyState === WebSocket.OPEN) ws.send(frame);
      }
    });
  }

  function createTarget(url?: string): string {
    const id = randomUUID();
    const win = new BrowserWindow({
      show: false,
      x: -32000,
      y: -32000,
      width: 1280,
      height: 800,
      webPreferences: {
        partition: PARTITION,
        sandbox: true,
        contextIsolation: true,
        nodeIntegration: false,
        // 隐藏窗口必须保持全速渲染（rAF/IntersectionObserver 是滚动驱动抓取的前提）
        backgroundThrottling: false,
        spellcheck: false,
      },
    });
    const target: ManagedTarget = { id, win, attached: false };
    targets.set(id, target);
    attachDebugger(target);
    win.once("closed", () => {
      targets.delete(id);
      pageSockets.delete(id);
      broadcastBrowser({ method: "Target.targetDestroyed", params: { targetId: id } });
    });
    void win.loadURL(url ?? "about:blank").catch((err: unknown) => {
      log(`[cdp-proxy] loadURL failed: ${String(err)}`);
    });
    return id;
  }

  function closeTarget(id: string): boolean {
    const target = targets.get(id);
    if (!target) return false;
    targets.delete(id);
    pageSockets.delete(id);
    if (!target.win.isDestroyed()) target.win.close();
    return true;
  }

  async function closeAllTargets(): Promise<void> {
    for (const id of [...targets.keys()]) closeTarget(id);
  }

  function targetJson(id: string): Record<string, unknown> | undefined {
    const target = targets.get(id);
    if (!target || target.win.isDestroyed()) return undefined;
    const wc = target.win.webContents;
    return {
      id,
      type: "page",
      title: wc.getTitle(),
      url: wc.getURL() || "about:blank",
      webSocketDebuggerUrl: `ws://127.0.0.1:${port}/devtools/page/${id}`,
    };
  }

  // ── browser 级 WS（Target.*） ─────────────────────────────────────────────
  const browserSockets = new Set<WebSocket>();
  function broadcastBrowser(frame: { method: string; params?: unknown }): void {
    const text = JSON.stringify(frame);
    for (const ws of browserSockets) {
      if (ws.readyState === WebSocket.OPEN) ws.send(text);
    }
  }

  function handleBrowserCommand(
    msg: { id?: number; method?: string; params?: Record<string, unknown> },
    ws: WebSocket,
  ): void {
    if (msg.id === undefined) return;
    const reply = (result: unknown): void => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ id: msg.id, result }));
    };
    const fail = (message: string): void => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ id: msg.id, error: { code: -32601, message } }));
      }
    };
    switch (msg.method) {
      case "Target.createTarget": {
        const url = typeof msg.params?.url === "string" ? msg.params.url : undefined;
        reply({ targetId: createTarget(url) });
        return;
      }
      case "Target.closeTarget": {
        const targetId = String(msg.params?.targetId ?? "");
        reply({ success: closeTarget(targetId) });
        return;
      }
      case "Target.getTargets":
        reply({
          targetInfos: [...targets.keys()]
            .map((id) => targetJson(id))
            .filter((t) => t !== undefined)
            .map((t) => ({ targetId: t.id, type: t.type, title: t.title, url: t.url })),
        });
        return;
      case "Target.activateTarget":
        // 隐藏窗口无焦点语义；滚动驱动的渲染不依赖窗口激活
        reply({});
        return;
      case "Target.setDiscoverTargets":
      case "Target.setAutoAttach":
        reply({});
        return;
      default:
        fail(`CDP proxy: unsupported browser method ${String(msg.method)}`);
    }
  }

  // ── page 级 WS（任意 CDP 命令 → webContents.debugger） ─────────────────────
  function handlePageConnection(socket: WebSocket, targetId: string): void {
    const target = targets.get(targetId);
    if (!target || target.win.isDestroyed()) {
      socket.close(4004, "unknown target");
      return;
    }
    attachDebugger(target);
    let set = pageSockets.get(targetId);
    if (!set) {
      set = new Set();
      pageSockets.set(targetId, set);
    }
    set.add(socket);
    socket.on("close", () => {
      set?.delete(socket);
      if (set && set.size === 0) pageSockets.delete(targetId);
    });
    socket.on("message", (data) => {
      let msg: { id?: number; method?: string; params?: Record<string, unknown> };
      try {
        msg = JSON.parse(decodeText(data)) as typeof msg;
      } catch {
        return;
      }
      const id = msg.id;
      if (id === undefined || typeof msg.method !== "string") return;
      const method = msg.method;
      const wc = target.win.webContents;
      if (wc.isDestroyed()) {
        sendError(socket, id, "target destroyed");
        return;
      }
      void wc.debugger
        .sendCommand(method, msg.params ?? {})
        .then((result) => {
          if (socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ id, result: result ?? {} }));
          }
        })
        .catch((err: unknown) => {
          sendError(socket, id, err instanceof Error ? err.message : String(err));
        });
    });
  }

  function sendError(socket: WebSocket, id: number, message: string): void {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ id, error: { code: -32000, message } }));
    }
  }

  // ── HTTP 端点（host 的 /json/* 与组件控制面） ──────────────────────────────
  let port = 0;
  const server: Server = createServer((req, res) => {
    const url = new URL(req.url ?? "/", "http://127.0.0.1");
    const json = (body: unknown): void => {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify(body));
    };
    if (url.pathname === "/json/version") {
      json({
        Browser: `Electron/${process.versions.electron}`,
        "Protocol-Version": "1.3",
        webSocketDebuggerUrl: `ws://127.0.0.1:${port}/devtools/browser`,
      });
      return;
    }
    if (url.pathname === "/json/list") {
      json(
        [...targets.keys()]
          .map((id) => targetJson(id))
          .filter((t) => t !== undefined),
      );
      return;
    }
    if (url.pathname.startsWith("/json/close/")) {
      closeTarget(decodeURIComponent(url.pathname.slice("/json/close/".length)));
      res.end("Closed");
      return;
    }
    if (url.pathname.startsWith("/json/activate/")) {
      res.end("Activated");
      return;
    }
    if (url.pathname === "/amoeba/close-targets" && req.method === "POST") {
      const n = targets.size;
      void closeAllTargets().then(() => json({ closed: n }));
      return;
    }
    if (url.pathname === "/amoeba/reset-profile" && req.method === "POST") {
      void (async () => {
        await closeAllTargets();
        await ses.clearStorageData();
        await ses.clearCache();
        json({ profile: PARTITION });
      })();
      return;
    }
    res.writeHead(404).end();
  });

  // ── WS 升级路由：/devtools/browser 与 /devtools/page/<id> ─────────────────
  const wss = new WebSocketServer({ noServer: true });
  server.on("upgrade", (req: IncomingMessage, socket, head) => {
    const url = new URL(req.url ?? "/", "http://127.0.0.1");
    if (!url.pathname.startsWith("/devtools/")) {
      socket.destroy();
      return;
    }
    wss.handleUpgrade(req, socket, head, (ws) => {
      if (url.pathname === "/devtools/browser") {
        browserSockets.add(ws);
        ws.on("close", () => browserSockets.delete(ws));
        ws.on("message", (data) => {
          try {
            handleBrowserCommand(JSON.parse(decodeText(data)) as never, ws);
          } catch {
            /* 畸形帧忽略 */
          }
        });
        // 新连接先发一次目标清单事件（与 Chromium 行为对齐，host 不依赖）
        ws.send(
          JSON.stringify({
            method: "Target.targetCreated",
            params: { targetInfo: { type: "browser" } },
          }),
        );
        return;
      }
      const targetId = decodeURIComponent(
        url.pathname.slice("/devtools/page/".length),
      );
      handlePageConnection(ws, targetId);
    });
  });

  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  port = (server.address() as AddressInfo).port;
  log(`[cdp-proxy] listening on 127.0.0.1:${port} (${PARTITION})`);

  const proxy: CdpProxy = {
    port,
    targetCount: () => targets.size,
    closeTargets: closeAllTargets,
    async resetProfile() {
      await closeAllTargets();
      await ses.clearStorageData();
      await ses.clearCache();
    },
    async selfTest() {
      const ua = await selfTest(port);
      log(`[cdp-proxy] self-test ok: ${ua}`);
      return ua;
    },
    async close() {
      await closeAllTargets();
      for (const ws of browserSockets) ws.terminate();
      browserSockets.clear();
      for (const set of pageSockets.values()) {
        for (const ws of set) ws.terminate();
      }
      pageSockets.clear();
      await new Promise<void>((resolve) => server.close(() => resolve()));
      wss.close();
    },
  };
  return proxy;
}

/** 用 ws 客户端走完整代理链路：browser WS → createTarget → page WS → evaluate。 */
async function selfTest(port: number): Promise<string> {
  const open = (url: string): Promise<WebSocket> =>
    new Promise((resolve, reject) => {
      const ws = new WebSocket(url, { perMessageDeflate: false });
      const timer = setTimeout(() => {
        ws.terminate();
        reject(new Error(`self-test 连接超时: ${url}`));
      }, SELF_TEST_TIMEOUT_MS);
      ws.once("open", () => {
        clearTimeout(timer);
        resolve(ws);
      });
      ws.once("error", (err) => {
        clearTimeout(timer);
        reject(err);
      });
    });
  const call = (
    ws: WebSocket,
    id: number,
    method: string,
    params: Record<string, unknown> = {},
  ): Promise<unknown> =>
    new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        ws.off("message", onMessage);
        reject(new Error(`self-test ${method} 超时`));
      }, SELF_TEST_TIMEOUT_MS);
      const onMessage = (data: RawData): void => {
        const msg = JSON.parse(decodeText(data)) as {
          id?: number;
          result?: unknown;
          error?: { message?: string };
        };
        if (msg.id !== id) return;
        clearTimeout(timer);
        ws.off("message", onMessage);
        if (msg.error) reject(new Error(msg.error.message ?? "CDP error"));
        else resolve(msg.result);
      };
      ws.on("message", onMessage);
      ws.send(JSON.stringify({ id, method, params }));
    });

  const browser = await open(`ws://127.0.0.1:${port}/devtools/browser`);
  try {
    const created = (await call(browser, 1, "Target.createTarget", {
      url: "about:blank",
    })) as { targetId?: string };
    const targetId = created.targetId;
    if (!targetId) throw new Error("self-test: Target.createTarget 未返回 targetId");
    const page = await open(`ws://127.0.0.1:${port}/devtools/page/${targetId}`);
    try {
      // host 的 armNetwork 每条连接都会发这两条（验证 Network domain 可用）
      await call(page, 2, "Network.enable");
      await call(page, 3, "Network.setBlockedURLs", { urls: ["*.mp4*"] });
      // cookie 读取（登录态核心，含 HttpOnly）
      const cookies = (await call(page, 4, "Network.getAllCookies")) as {
        cookies?: unknown[];
      };
      if (!Array.isArray(cookies.cookies)) {
        throw new Error("self-test: Network.getAllCookies 返回形状异常");
      }
      const evaluated = (await call(page, 5, "Runtime.evaluate", {
        expression: "navigator.userAgent",
        returnByValue: true,
      })) as { result?: { value?: string } };
      const ua = evaluated.result?.value ?? "";
      if (!ua.includes("Chrome/") || ua.includes("Electron/")) {
        throw new Error(`self-test: UA 未洗白（${ua}）`);
      }
      // 真实页面导航（data: URL，无网络依赖）再 evaluate；导航异步，轮询标题
      await call(page, 6, "Page.navigate", {
        url: "data:text/html,<title>cdp-probe</title><p>ok</p>",
      });
      let nextId = 7;
      let title = "";
      const deadline = Date.now() + 5_000;
      while (Date.now() < deadline) {
        const r = (await call(page, nextId++, "Runtime.evaluate", {
          expression: "document.title",
          returnByValue: true,
        })) as { result?: { value?: string } };
        title = r.result?.value ?? "";
        if (title === "cdp-probe") break;
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
      if (title !== "cdp-probe") {
        throw new Error(`self-test: 导航后 evaluate 异常（${title}）`);
      }
      await call(browser, nextId++, "Target.closeTarget", { targetId });
      return ua;
    } finally {
      page.terminate();
    }
  } finally {
    browser.terminate();
  }
}
