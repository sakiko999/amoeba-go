/**
 * media-manager 独立应用入口：窗口 + 协议 + 探针。
 * 媒体后端 IPC 在 backend.ts（同一份代码也被宿主页面扩展复用）；这里只负责窗口壳。
 * 开发：ELECTRON_RENDERER_URL（electron-vite dev）；生产：app:// 映射 out/renderer。
 * `--smoke`：无头冒烟，renderer 加载 + preload API 面检查后 exit 0。
 * `--probe-bangumi` / `--probe-bangumi-apply`：真实网络探针（见 probe.ts）。
 */

import { app, BrowserWindow, ipcMain, Menu, protocol, session } from "electron";
import type { Session, WebContents } from "electron";
import { readFile } from "node:fs/promises";
import { isAbsolute, join, relative } from "node:path";
import { HOST_STATE_CHANNELS } from "@host/sdk";
import { registerMediaBackend } from "./backend";
import { runBangumiApplyProbe, runBangumiProbe } from "./probe";

const SMOKE = process.argv.includes("--smoke");
const DEV_URL = process.env.ELECTRON_RENDERER_URL;
// 与 @demo/ui theme.css 对齐：顶栏是 bg-card（dark --card = #171717），窗口按钮区必须同色；
// 高度比 44px 顶栏少 1px，让原生 overlay 底下露出 DOM 的 border-bottom
const OVERLAY = { color: "#171717", symbolColor: "#fafafa", height: 43 };

// 发布后的 userData 路径固定为 %APPDATA%/media-manager（不随 scoped 包名变化）
app.setName("media-manager");

let win: BrowserWindow | undefined;

const MIME: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".webp": "image/webp",
  ".ico": "image/x-icon",
  ".woff2": "font/woff2",
  ".woff": "font/woff",
  ".ttf": "font/ttf",
};

// app:// scheme 必须在 app ready 之前注册
protocol.registerSchemesAsPrivileged([
  {
    scheme: "app",
    privileges: { standard: true, secure: true, supportFetchAPI: true },
  },
]);

/** demo 不需要任何 Web 权限：Electron 默认是全授予，这里改成默认全拒。 */
function denyAllPermissions(ses: Session): void {
  ses.setPermissionRequestHandler((_wc, _permission, callback) =>
    callback(false),
  );
  ses.setPermissionCheckHandler(() => false);
}

/**
 * app:// → out/renderer；必须按 session 注册（协议方案同 electron-tabs-demo）。
 * 直接 readFile 而非 net.fetch(file://)：Node fs 支持 asar，打包后同样可读。
 */
async function serveAppFile(root: string, request: Request): Promise<Response> {
  try {
    const path = decodeURIComponent(new URL(request.url).pathname);
    const file = join(root, path);
    const rel = relative(root, file);
    if (rel === "" || rel.startsWith("..") || isAbsolute(rel)) {
      return new Response(null, { status: 404 });
    }
    const buf = await readFile(file);
    const ext = file.slice(file.lastIndexOf(".")).toLowerCase();
    return new Response(buf, {
      headers: { "content-type": MIME[ext] ?? "application/octet-stream" },
    });
  } catch {
    return new Response(null, { status: 404 });
  }
}

function registerAppProtocol(ses: Session): void {
  const root = join(app.getAppPath(), "out", "renderer");
  ses.protocol.handle("app", (request) => serveAppFile(root, request));
}

/** 统一加固：F12 开 devtools；拒绝一切导航与 window.open —— 只装本地打包页面。 */
function hardenWebContents(wc: WebContents): void {
  wc.on("before-input-event", (_event, input) => {
    if (input.type === "keyDown" && input.key === "F12") {
      if (wc.isDevToolsOpened()) wc.closeDevTools();
      else wc.openDevTools({ mode: "detach" });
    }
  });
  const deny = (event: { preventDefault(): void }) => event.preventDefault();
  wc.on("will-navigate", deny);
  wc.on("will-redirect", deny);
  wc.setWindowOpenHandler(() => ({ action: "deny" }));
}

async function main(): Promise<void> {
  const started = Date.now();
  if (SMOKE) app.disableHardwareAcceleration();
  // 单实例：打包版双击两次会争用 userData / cache.db（SQLITE_BUSY）
  if (!SMOKE && !app.requestSingleInstanceLock()) {
    app.quit();
    return;
  }
  await app.whenReady();
  app.setAppUserModelId("com.sakiko999.media-manager");
  if (!SMOKE) Menu.setApplicationMenu(null);

  const applyProbeIdx = process.argv.indexOf("--probe-bangumi-apply");
  if (applyProbeIdx >= 0) {
    const [dir, id] = process.argv.slice(applyProbeIdx + 1);
    await runBangumiApplyProbe(dir ?? "", id ?? "");
    app.exit(0);
    return;
  }
  const probeIdx = process.argv.indexOf("--probe-bangumi");
  if (probeIdx >= 0) {
    const [id, keyword] = process.argv.slice(probeIdx + 1);
    await runBangumiProbe(id ?? "", keyword);
    app.exit(0);
    return;
  }

  registerMediaBackend({ getWindow: () => win });

  // 独立应用没有宿主状态源：给页面 preload 的 hostState 桥一个固定 dark，
  // 同时避免 sendSync 无监听时的 Electron 警告
  const standaloneState = { theme: "dark" } as const;
  ipcMain.on(HOST_STATE_CHANNELS.sync, (e) => {
    e.returnValue = standaloneState;
  });
  ipcMain.handle(HOST_STATE_CHANNELS.get, () => standaloneState);
  // preload 用它区分“宿主页面”与“独立应用”，否则 sendSync 会打印无监听警告
  ipcMain.on("app:whoami", (e) => {
    e.returnValue = { role: "unknown" };
  });

  registerAppProtocol(session.defaultSession);
  denyAllPermissions(session.defaultSession);
  app.on("session-created", (ses) => {
    registerAppProtocol(ses);
    denyAllPermissions(ses);
  });

  win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 720,
    minHeight: 480,
    title: "Media Manager",
    backgroundColor: "#0a0a0a",
    // 无系统标题栏：renderer 的 44px 顶栏即标题栏（drag 区 + 原生 overlay 窗口按钮）
    titleBarStyle: "hidden",
    titleBarOverlay: OVERLAY,
    webPreferences: {
      preload: join(app.getAppPath(), "out", "preload", "index.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
    },
  });
  hardenWebContents(win.webContents);

  const url = DEV_URL ? `${DEV_URL}/index.html` : "app://r/index.html";
  const window_ = win;
  const loaded = new Promise<void>((resolve, reject) => {
    window_.webContents.once("did-finish-load", () => resolve());
    window_.webContents.once(
      "did-fail-load",
      (_e, code, desc, _u, isMainFrame) => {
        if (isMainFrame) reject(new Error(`load failed: ${desc} (${code})`));
      },
    );
  });
  await win.loadURL(url);
  if (!SMOKE) {
    // dev 任务（.vscode/tasks.json）以这行作为 inspector 就绪信号
    console.log(
      `[startup] media-manager ready in ${Date.now() - started}ms ` +
        `(inspector ${process.env.V8_INSPECTOR_PORT ?? "off"})`,
    );
  }

  if (SMOKE) {
    const timeout = new Promise<never>((_, reject) => {
      setTimeout(() => reject(new Error("smoke timeout after 15s")), 15_000);
    });
    await Promise.race([loaded, timeout]);
    // preload 必须暴露完整 IPC 面：对话框在点击后才渲染，加载成功不代表接口可用
    const apiKeys = await win.webContents.executeJavaScript(
      "Object.keys(window.media ?? {})",
    );
    if (!Array.isArray(apiKeys) || apiKeys.length < 13) {
      throw new Error(`preload API incomplete: ${String(apiKeys)}`);
    }
    console.log(
      `SMOKE OK: renderer loaded, media API ${apiKeys.length} methods`,
    );
    app.exit(0);
  }
}

app.on("second-instance", () => {
  if (win === undefined) return;
  if (win.isMinimized()) win.restore();
  win.focus();
});

app.on("window-all-closed", () => app.quit());

main().catch((err: unknown) => {
  console.error("main crashed:", err);
  app.exit(1);
});
