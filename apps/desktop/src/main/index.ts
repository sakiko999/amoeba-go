/**
 * 主进程入口：注册 IPC → 建窗口 → TabManager → 载入 host ui → 打开第一个 tab。
 * `--smoke`：无头冒烟模式，三个 tab 都成功加载后 exit 0；did-fail-load 立即判失败。
 * renderer 加载：dev 走 electron-vite dev server，生产走 app:// 自定义协议。
 */

import {
  app,
  BrowserWindow,
  ipcMain,
  Menu,
  net,
  protocol,
  session,
} from "electron";
import type { Session } from "electron";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import os from "node:os";
import type { SystemInfo, TabSummary, Theme } from "@tabs/sdk";
import { TAB_KINDS } from "./registry";
import { TabManager, hardenWebContents, toggleDevtools } from "./tabs";
import { hostUrl } from "./renderTarget";

const SMOKE = process.argv.includes("--smoke");
const INSPECT_ARG = "--inspect=9229";

let manager: TabManager | undefined;
let win: BrowserWindow | undefined;

/* ---------- 主题（唯一状态源；demo 不持久化，重启回深色） ---------- */
let theme: Theme = "dark";
/** titleBarOverlay 颜色随主题走（窗口按钮为原生 overlay，cherry Windows 同款） */
const OVERLAY_COLORS: Record<Theme, { color: string; symbolColor: string }> = {
  dark: { color: "#1a1b26", symbolColor: "#c0caf5" },
  light: { color: "#e6e7f2", symbolColor: "#3b3f5c" },
};

ipcMain.handle("theme:get", () => theme);
ipcMain.on("theme:toggle", (e) => {
  // 只有 host 的 tab 条有切换入口，tab 页面无权改全局主题
  if (
    !win ||
    win.isDestroyed() ||
    manager?.whoami(e.sender.id).role !== "host"
  ) {
    return;
  }
  theme = theme === "dark" ? "light" : "dark";
  win.setTitleBarOverlay(OVERLAY_COLORS[theme]);
  for (const wc of [win.webContents, ...manager.tabWebContents()]) {
    wc.send("theme:changed", theme);
  }
});

/* ---------- host（tab 条）侧 ---------- */
ipcMain.handle("tabs:list-kinds", () => TAB_KINDS);
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
  const inspecting = process.argv.includes(INSPECT_ARG);
  const menu = Menu.buildFromTemplate([
    {
      // 主进程是 Node 进程没有 DevTools，只能开 inspector；重启附加 --inspect 后在浏览器 chrome://inspect 连接
      label: inspecting
        ? "主进程 inspector 已开启（chrome://inspect 连 9229）"
        : "调试主进程（重启并开启 --inspect=9229）",
      enabled: !inspecting,
      click: () => {
        console.log(
          "relaunching with --inspect=9229; attach via chrome://inspect",
        );
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

// app:// → out/renderer（须在 app ready 前注册 scheme）
protocol.registerSchemesAsPrivileged([
  {
    scheme: "app",
    privileges: { standard: true, secure: true, supportFetchAPI: true },
  },
]);

/** app:// → out/renderer；必须按 session 注册：每个 tab 实例的独立 partition 是另一个 session。 */
function registerAppProtocol(ses: Session): void {
  const root = join(app.getAppPath(), "out", "renderer");
  ses.protocol.handle("app", (request) => {
    const path = decodeURIComponent(new URL(request.url).pathname);
    const file = join(root, path);
    // 防目录穿越：只允许 renderer 根之内的文件
    if (!file.startsWith(root)) return new Response(null, { status: 404 });
    return net.fetch(pathToFileURL(file).toString());
  });
}

async function main(): Promise<void> {
  if (SMOKE) app.disableHardwareAcceleration(); // 冒烟环境可能无 GPU，纯软件渲染更稳
  await app.whenReady();

  registerAppProtocol(session.defaultSession);
  denyAllPermissions(session.defaultSession);
  // 每个 tab 实例的独立 partition session 也要覆盖（协议 + 权限）
  app.on("session-created", (ses) => {
    registerAppProtocol(ses);
    denyAllPermissions(ses);
  });

  win = new BrowserWindow({
    width: 1080,
    height: 720,
    minWidth: 640,
    minHeight: 420,
    title: "Electron Tabs Demo",
    backgroundColor: "#16161e",
    // 无系统标题栏：tab 条即标题栏（drag 区 + 原生 overlay 窗口按钮，高 44 与 tab 条一致）
    titleBarStyle: "hidden",
    titleBarOverlay: { ...OVERLAY_COLORS.dark, height: 44 },
    webPreferences: {
      preload: join(app.getAppPath(), "out", "preload", "index.js"),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
    },
  });
  manager = new TabManager(win);
  hardenWebContents(win.webContents);

  await win.loadURL(hostUrl());

  if (SMOKE) {
    const loads: Promise<void>[] = [];
    for (const kind of TAB_KINDS) {
      const summary = manager.create(kind.id);
      const wc = summary
        ? manager.webContentsOf(summary.instanceId)
        : undefined;
      if (!summary || !wc)
        throw new Error(`smoke: cannot create tab ${kind.id}`);
      loads.push(
        new Promise<void>((resolve, reject) => {
          wc.once("did-finish-load", () => resolve());
          // 加载失败时错误页同样会触发 did-finish-load，必须显式捕获 fail 才不算成功
          wc.once("did-fail-load", (_e, code, desc, _url, isMainFrame) => {
            if (isMainFrame) {
              reject(
                new Error(`smoke: ${kind.id} load failed: ${desc} (${code})`),
              );
            }
          });
        }),
      );
    }
    const timeout = new Promise<never>((_, reject) => {
      setTimeout(() => reject(new Error("smoke timeout after 15s")), 15_000);
    });
    await Promise.race([Promise.all(loads), timeout]);
    console.log(`SMOKE OK: ${loads.length}/${TAB_KINDS.length} tabs loaded`);
    app.exit(0);
    return;
  }

  // 默认打开第一个 tab（此刻 host 已 did-finish-load，broadcast 一定能到达）
  const first = TAB_KINDS[0];
  if (first) manager.create(first.id);
}

app.on("window-all-closed", () => app.quit());

main().catch((err: unknown) => {
  console.error("main crashed:", err);
  app.exit(1);
});
