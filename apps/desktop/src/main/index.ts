/**
 * 主进程入口：页面注册表 → IPC → 窗口 → TabManager → host ui。
 * 静态 tab（packages/tab-*）走编译期 MPA；runtime 页面（page://）由 PageRegistry 扫描
 * 内置 `pages/` 与用户数据目录，支持安装/卸载/重载，无需重启宿主。
 * `--smoke`：无头冒烟，静态 tab 与已注册页面全部加载后 exit 0；did-fail-load 立即失败。
 */

import { app, BrowserWindow, ipcMain, Menu, protocol, session } from "electron";
import type { Session, WebContents } from "electron";
import { appendFile } from "node:fs/promises";
import { join } from "node:path";
import os from "node:os";
import { HOST_STATE_CHANNELS } from "@host/sdk";
import type { SystemInfo, TabSummary, Theme } from "@host/sdk";
import { createPageActions } from "./pages/actions";
import { registerPageProtocol } from "./pages/protocol";
import { PageRegistry } from "./pages/registry";
import { TAB_KINDS } from "./registry";
import { HostStateStore } from "./state";
import {
  TABBAR_HEIGHT,
  TabManager,
  hardenWebContents,
  toggleDevtools,
} from "./tabs";
import { hostUrl } from "./renderTarget";
import { isInside } from "./pages/manifest";
import { serveFile } from "./staticFiles";

const SMOKE = process.argv.includes("--smoke");
const INSPECT_ARG = "--inspect=9229";

// 冒烟必须隔离 userData：外部安装的页面/持久分区不能影响结果
if (SMOKE) {
  app.setPath(
    "userData",
    join(app.getPath("temp"), `tabs-smoke-${process.pid}`),
  );
}

// 打包后 GUI 应用没有 stdout：smoke 或显式 TABS_SMOKE_LOG 时把主进程日志落到文件
const LOG_FILE =
  process.env.TABS_SMOKE_LOG ??
  (SMOKE
    ? join(app.getPath("temp"), `tabs-smoke-${process.pid}.log`)
    : undefined);
if (LOG_FILE !== undefined) {
  // tee：终端照常输出；打包后 GUI 无 stdout 时以文件为准
  const toFile = (level: string) => {
    const write = (...args: unknown[]) => {
      void appendFile(
        LOG_FILE,
        `${new Date().toISOString()} ${level} ${args.map(String).join(" ")}\n`,
      ).catch(() => undefined);
    };
    return write;
  };
  const log = console.log.bind(console);
  const error = console.error.bind(console);
  console.log = (...args: unknown[]) => {
    log(...args);
    toFile("log")(...args);
  };
  console.error = (...args: unknown[]) => {
    error(...args);
    toFile("err")(...args);
  };
  process.on("uncaughtException", (err) => {
    console.error("uncaughtException", err.stack ?? String(err));
    process.exit(1);
  });
}

let manager: TabManager | undefined;
let win: BrowserWindow | undefined;
let pages: PageRegistry | undefined;

/* ---------- 宿主状态（唯一状态源；广播给 host、静态 tab 与 runtime 页面） ---------- */
const stateStore = new HostStateStore();
/** titleBarOverlay 颜色随主题走；与 @demo/ui theme.css 的 --sidebar/--sidebar-foreground 对齐 */
const OVERLAY_COLORS: Record<Theme, { color: string; symbolColor: string }> = {
  dark: { color: "#171717", symbolColor: "#fafafa" },
  light: { color: "#fafafa", symbolColor: "#0a0a0a" },
};
/** 让 overlay 高度比 tab 条少 1px，底边露出 DOM 的 border-bottom（原生 overlay 会盖住 DOM 绘制） */
const OVERLAY_HEIGHT = TABBAR_HEIGHT - 1;
/** 窗口底色与 theme.css 的 --background 对齐（主题切换后露出的颜色） */
const BACKGROUND_COLORS: Record<Theme, string> = {
  dark: "#0a0a0a",
  light: "#ffffff",
};

const isHost = (wcId: number): boolean => manager?.whoami(wcId).role === "host";

/** 整份状态广播；theme:changed 保留给尚未迁移到 state API 的旧 tab */
function broadcastState(): void {
  if (!win || win.isDestroyed()) return;
  const state = stateStore.get();
  win.setTitleBarOverlay({
    ...OVERLAY_COLORS[state.theme],
    height: OVERLAY_HEIGHT,
  });
  win.setBackgroundColor(BACKGROUND_COLORS[state.theme]);
  for (const wc of [win.webContents, ...(manager?.tabWebContents() ?? [])]) {
    wc.send(HOST_STATE_CHANNELS.changed, state);
    wc.send("theme:changed", state.theme);
  }
}

// sync 供页面 preload 首帧同步取值；页面只读，写入仅限 host（tab 条）
ipcMain.on(HOST_STATE_CHANNELS.sync, (e) => {
  e.returnValue = stateStore.get();
});
ipcMain.handle(HOST_STATE_CHANNELS.get, () => stateStore.get());
ipcMain.on(HOST_STATE_CHANNELS.set, (e, patch: unknown) => {
  if (!isHost(e.sender.id) || patch === null || typeof patch !== "object") {
    return;
  }
  const theme = (patch as { theme?: unknown }).theme;
  if (theme !== "dark" && theme !== "light") return;
  stateStore.set({ theme });
});
// 兼容旧 tab / host 的 theme API
ipcMain.handle("theme:get", () => stateStore.get().theme);
ipcMain.on("theme:toggle", (e) => {
  // 只有 host 的 tab 条有切换入口，tab 页面无权改全局主题
  if (!isHost(e.sender.id)) return;
  stateStore.set({
    theme: stateStore.get().theme === "dark" ? "light" : "dark",
  });
});

/* ---------- host（tab 条）侧 ---------- */
ipcMain.handle("tabs:list-kinds", () => manager?.kinds ?? TAB_KINDS);
ipcMain.handle("tabs:create", (_e, kind: string): TabSummary => {
  const summary = manager?.create(kind);
  if (!summary) throw new Error(`unknown tab kind: ${kind}`);
  return summary;
});
ipcMain.on("tabs:new-tab-menu", () => manager?.popupNewTabMenu());
ipcMain.on("tabs:context-menu", (_e, instanceId: string) =>
  manager?.popupTabContextMenu(instanceId),
);
ipcMain.on("tabs:activate", (_e, instanceId: string) =>
  manager?.activate(instanceId),
);
ipcMain.on("tabs:close", (_e, instanceId: string) =>
  manager?.close(instanceId),
);
ipcMain.on("tabs:move", (_e, instanceId: string, overInstanceId: string) =>
  manager?.move(instanceId, overInstanceId),
);

/* ---------- 调试入口（host tab 条的 bug 按钮弹出） ---------- */
ipcMain.on("debug:menu", (e) => {
  const window = win;
  if (
    !window ||
    window.isDestroyed() ||
    manager?.whoami(e.sender.id).role !== "host"
  ) {
    return;
  }
  const active = manager.activeWebContents();
  // electron-vite 的 --inspect=<port> 会设置该 env 并由 Electron 子进程继承
  const inspectPort = process.env.V8_INSPECTOR_PORT;
  const inspecting = inspectPort !== undefined;
  // dev 下 app.relaunch 会脱离 electron-vite 的管理，触发 Vite 重新优化时 tab 加载失败
  const devMode = process.env.ELECTRON_RENDERER_URL !== undefined;
  const menu = Menu.buildFromTemplate([
    {
      // 主进程是 Node 进程没有 DevTools，只能开 inspector；重启附加 --inspect 后在浏览器 chrome://inspect 连接
      label: inspecting
        ? `主进程 inspector 已开启（chrome://inspect 连 ${inspectPort}）`
        : "调试主进程（重启并开启 --inspect=9229）",
      enabled: !devMode && !inspecting,
      click: () => {
        if (devMode || inspecting) return;
        console.log(
          "relaunching with --inspect=9229; attach via chrome://inspect",
        );
        // 让 relaunch 的新进程也能通过 env 判断 inspector 已开启
        process.env.V8_INSPECTOR_PORT = "9229";
        app.relaunch({ args: [...process.argv.slice(1), INSPECT_ARG] });
        app.exit(0);
      },
    },
    {
      label: "调试启动器界面（tab 条 host 页面）",
      click: () => toggleDevtools(window.webContents),
    },
    {
      label: "调试内容页面（当前活动 tab）",
      enabled: active !== undefined,
      click: () => {
        if (active) toggleDevtools(active);
      },
    },
  ]);
  menu.popup({ window });
});

/* ---------- tab 侧（一律用 sender 定位调用者，不信任页面传来的 instanceId） ---------- */
ipcMain.handle("tab:set-title", (e, title: string) =>
  manager?.setTitleByWebContents(e.sender.id, title),
);
ipcMain.handle("tab:close-self", (e) =>
  manager?.closeByWebContents(e.sender.id),
);
ipcMain.handle("tab:ping", (e, value: string) => {
  const who = manager?.whoami(e.sender.id);
  return `pong:${value} @ ${who?.role === "tab" ? who.kind : "?"}`;
});
ipcMain.handle("tab:system-info", (): SystemInfo => ({
  platform: process.platform,
  arch: process.arch,
  cpus: os.cpus().length,
  memoryGb:
    Math.round((process.getSystemMemoryInfo().total / 1024 / 1024) * 10) / 10,
  electron: process.versions.electron,
  chrome: process.versions.chrome,
  node: process.versions.node,
}));

/* ---------- preload 启动时的角色询问（sendSync；必须在任何窗口创建前注册） ---------- */
ipcMain.on("app:whoami", (e) => {
  e.returnValue = manager ? manager.whoami(e.sender.id) : { role: "unknown" };
});

/** demo 不需要任何 Web 权限：Electron 默认是全授予，这里改成默认全拒。 */
function denyAllPermissions(ses: Session): void {
  ses.setPermissionRequestHandler((_wc, _permission, callback) =>
    callback(false),
  );
  ses.setPermissionCheckHandler(() => false);
}

// app:// 静态 tab 与 page:// runtime 页面（须在 app ready 前注册 scheme）
protocol.registerSchemesAsPrivileged([
  {
    scheme: "app",
    privileges: { standard: true, secure: true, supportFetchAPI: true },
  },
  {
    scheme: "page",
    privileges: {
      standard: true,
      secure: true,
      supportFetchAPI: true,
      corsEnabled: true,
    },
  },
]);

/** app:// → out/renderer；必须按 session 注册：每个 tab 实例的独立 partition 是另一个 session。
 *  打包后资源在 app.asar 内，用 fs.readFile（serveFile）而不是 net.fetch(file://)。 */
function registerAppProtocol(ses: Session): void {
  const root = join(app.getAppPath(), "out", "renderer");
  ses.protocol.handle("app", (request) => {
    const path = decodeURIComponent(new URL(request.url).pathname);
    const file = join(root, path);
    // 防目录穿越：只允许 renderer 根之内的文件
    if (!isInside(root, file)) return new Response(null, { status: 404 });
    return serveFile(file);
  });
}

function registerSessionProtocols(ses: Session): void {
  registerAppProtocol(ses);
  if (pages !== undefined) registerPageProtocol(ses, pages);
  denyAllPermissions(ses);
}

async function main(): Promise<void> {
  console.log(`smoke=${SMOKE} argv=${process.argv.join(" ")}`);
  if (SMOKE) app.disableHardwareAcceleration(); // 冒烟环境可能无 GPU，纯软件渲染更稳
  await app.whenReady();
  const started = Date.now();
  stateStore.load();

  pages = await PageRegistry.create({
    builtinDir: join(app.getAppPath(), "pages"),
    externalDir: join(app.getPath("userData"), "pages"),
    reservedIds: TAB_KINDS.map((kind) => kind.id),
    getWindow: () => win,
    log: (...args) => console.log("[pages]", ...args),
  });
  const scanMs = Date.now() - started;

  // 先装 session 策略（协议 + 权限），再建窗，最后才激活页面 main；
  // 页面 activate 里创建 session 也会命中 session-created 监听
  registerSessionProtocols(session.defaultSession);
  // 每个 tab/页面实例的 partition session 也要覆盖（协议 + 权限）
  app.on("session-created", (ses) => registerSessionProtocols(ses));

  win = new BrowserWindow({
    width: 1080,
    height: 720,
    minWidth: 640,
    minHeight: 420,
    title: "Electron Tabs Demo",
    // 白屏阶段整窗都用 tab 条色，右上角 overlay 才不出现色块；加载后 host UI 会覆盖整窗
    backgroundColor: OVERLAY_COLORS[stateStore.get().theme].color,
    // 无系统标题栏：tab 条即标题栏（drag 区 + 原生 overlay 窗口按钮，高 44 与 tab 条一致）
    titleBarStyle: "hidden",
    titleBarOverlay: {
      ...OVERLAY_COLORS[stateStore.get().theme],
      height: OVERLAY_HEIGHT,
    },
    webPreferences: {
      preload: join(app.getAppPath(), "out", "preload", "index.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
    },
  });

  const actions = createPageActions({
    pages,
    getWindow: () => win,
    closeTabs: (pageId) => manager?.closeByPage(pageId),
    openPage: (pageId) => {
      manager?.create(pageId);
    },
    onChanged: () => manager?.notifyKindsChanged(),
  });
  manager = new TabManager(win, pages, actions);
  hardenWebContents(win.webContents);
  // 状态变更 → overlay + 所有 webContents（host / 静态 tab / runtime 页面）
  stateStore.subscribe(() => broadcastState());

  await win.loadURL(hostUrl());
  console.log(
    `[startup] registry ${scanMs}ms, host ui ${Date.now() - started}ms ` +
      `(inspector ${process.env.V8_INSPECTOR_PORT ?? "off"})`,
  );

  // 窗口/session 策略就绪后再激活页面 main；非 smoke 走后台，避免阻塞首屏
  const activationStarted = Date.now();
  const activation = pages.activateAll().then(() => {
    manager?.notifyKindsChanged();
    // 只统计激活本身：不要把它和 host UI 的加载时间混在一起
    console.log(
      `[startup] pages activated in ${Date.now() - activationStarted}ms`,
    );
  });

  if (SMOKE) {
    await activation;
    const staticIds = TAB_KINDS.map((kind) => kind.id);
    if (pages.list().length === 0) {
      console.log(
        "smoke: no pages registered (run bun run pages:prepare to install media-manager)",
      );
    }
    const failedPages = pages.list().filter((page) => page.state === "error");
    if (failedPages.length > 0) {
      throw new Error(
        `smoke: page main failed: ${failedPages
          .map((page) => `${page.id}: ${page.error ?? "unknown"}`)
          .join("; ")}`,
      );
    }
    const pageIds = pages
      .list()
      .filter((page) => page.state !== "inactive")
      .map((page) => page.id);
    const loads: Promise<void>[] = [];
    let mediaWc: WebContents | undefined;
    for (const id of [...staticIds, ...pageIds]) {
      const tabStarted = Date.now();
      const summary = manager.create(id);
      const wc = summary
        ? manager.webContentsOf(summary.instanceId)
        : undefined;
      if (!summary || !wc) throw new Error(`smoke: cannot create tab ${id}`);
      if (summary.loading !== true) {
        throw new Error(`smoke: ${id} should start in loading state`);
      }
      if (id === "media-manager") mediaWc = wc;
      loads.push(
        new Promise<void>((resolve, reject) => {
          wc.once("did-finish-load", () => {
            const state = manager
              ?.summaries()
              .find((t) => t.instanceId === summary.instanceId);
            if (state?.loading !== false) {
              reject(
                new Error(`smoke: ${id} still loading after did-finish-load`),
              );
              return;
            }
            console.log(`[timing] tab ${id}: ${Date.now() - tabStarted}ms`);
            resolve();
          });
          // 加载失败时错误页同样会触发 did-finish-load，必须显式捕获 fail 才不算成功
          wc.once("did-fail-load", (_e, code, desc, _url, isMainFrame) => {
            if (isMainFrame) {
              reject(new Error(`smoke: ${id} load failed: ${desc} (${code})`));
            }
          });
        }),
      );
    }
    const timeout = new Promise<never>((_, reject) => {
      setTimeout(() => reject(new Error("smoke timeout after 30s")), 30_000);
    });
    await Promise.race([Promise.all(loads), timeout]);

    // 过渡页数据流：loading 投影必须到达 host。DOM 断言在无头环境有竞态
    // （页面加载极快时 React 会把 loading=true/false 合并成一次提交），
    // 这里查 renderer 的观测点 window.__loadingSeen
    const loadingSeen = (await win?.webContents.executeJavaScript(
      "window.__loadingSeen ?? []",
    )) as string[] | undefined;
    if (!loadingSeen?.includes("media-manager")) {
      throw new Error(
        `smoke: loading state never reached host (${JSON.stringify(loadingSeen)})`,
      );
    }
    console.log("SMOKE: loading projection ok", loadingSeen);

    // 页面 bridge 冒烟：pageBridge 必须与页面自有 preload 叠加（window.tab + hostState）
    if (mediaWc !== undefined) {
      const bridge = (await mediaWc.executeJavaScript(
        "({ kind: window.tab?.kind, theme: window.hostState?.initial?.theme, media: typeof window.media?.scanLibrary === 'function' })",
      )) as { kind?: string; theme?: string; media?: boolean };
      if (
        bridge.kind !== "media-manager" ||
        bridge.media !== true ||
        typeof bridge.theme !== "string"
      ) {
        throw new Error(
          `smoke: page bridge incomplete: ${JSON.stringify(bridge)}`,
        );
      }
      console.log("SMOKE: page bridge ok", bridge);
    }

    // 状态透传冒烟：切换宿主主题，media 页面必须跟随（页面 preload 订阅 host:state）
    if (mediaWc !== undefined) {
      const readTheme = (): Promise<unknown> =>
        mediaWc?.executeJavaScript("document.documentElement.dataset.theme");
      const before = await readTheme();
      const next = before === "dark" ? "light" : "dark";
      stateStore.set({ theme: next });
      await new Promise((resolve) => setTimeout(resolve, 250));
      const after = await readTheme();
      if (after !== next) {
        throw new Error(
          `smoke: theme not propagated to media page (expected ${next}, got ${String(after)})`,
        );
      }
      stateStore.set({ theme: before === "dark" ? "dark" : "light" });
      console.log("SMOKE: theme propagated to media page");
    }

    console.log(
      `SMOKE OK: ${loads.length} tabs loaded (${staticIds.length} static + ${pageIds.length} pages)`,
    );
    void pages.dispose();
    app.exit(0);
    return;
  }

  void activation.catch((err: unknown) =>
    console.error("[pages] activation failed:", err),
  );

  // 默认打开第一个 tab（此刻 host 已 did-finish-load，broadcast 一定能到达）
  const first = TAB_KINDS[0];
  if (first) manager.create(first.id);
}

app.on("before-quit", () => {
  void pages?.dispose();
});
app.on("window-all-closed", () => app.quit());

main().catch((err: unknown) => {
  console.error("main crashed:", err);
  app.exit(1);
});
