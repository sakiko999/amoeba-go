/**
 * TabManager：tab 实例 = 一个 WebContentsView（独立 webContents / preload / 内存 partition / 生命周期），
 * 挂在 BrowserWindow 的 contentView 上，叠在 host ui 之上：顶部 40px 留给 tab 条，其余归活动 tab。
 * 所有 tab 状态的唯一权威源在这里，host 页面只做投影（tabs:changed）。
 */

import { Menu, WebContentsView, app } from "electron";
import { join } from "node:path";
import type { BrowserWindow, WebContents } from "electron";
import type { TabManifest, TabSummary, Whoami } from "@tabs/sdk";
import { TAB_KINDS } from "./registry";
import { tabUrl } from "./renderTarget";

/** host 页面里 tab 条的高度；主进程与 ui/index.html 里保持一致。 */
const TABBAR_HEIGHT = 44;

interface TabEntry {
  instanceId: string;
  kind: string;
  title: string;
  view: WebContentsView;
  /** 创建时固化 webContents 引用与 id：destroyed 之后 view.webContents 不可再访问 */
  wc: WebContents;
  wcId: number;
}

export class TabManager {
  private readonly win: BrowserWindow;
  private readonly entries = new Map<string, TabEntry>();
  private readonly byWebContents = new Map<number, string>();
  private activeId: string | undefined;
  private seq = 0;

  constructor(win: BrowserWindow) {
    this.win = win;
    win.on("resize", () => this.layout());
    // host 重载（Ctrl+R / devtools reload）后重发整份状态，避免 tab 条与主进程脱节
    win.webContents.on("did-finish-load", () => this.broadcast());
  }

  get kinds(): readonly TabManifest[] {
    return TAB_KINDS;
  }

  create(kind: string): TabSummary | undefined {
    const manifest = TAB_KINDS.find((m) => m.id === kind);
    if (!manifest) return undefined;

    const instanceId = `${kind}-${++this.seq}`;
    // 独立内存 partition：实例间 storage 互不可见（不持久，随 tab 关闭即清）
    const view = new WebContentsView({
      webPreferences: {
        preload: join(app.getAppPath(), "out", "preload", "index.js"),
        partition: instanceId,
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
      },
    });
    const entry: TabEntry = {
      instanceId,
      kind,
      title: manifest.title,
      view,
      wc: view.webContents,
      wcId: view.webContents.id,
    };
    this.entries.set(instanceId, entry);
    this.byWebContents.set(entry.wcId, instanceId);
    this.win.contentView.addChildView(view);
    hardenWebContents(entry.wc);
    // 页面自己 window.close() 等外部销毁也走同一套清理
    entry.wc.once("destroyed", () => this.removeEntry(entry));
    entry.wc.loadURL(tabUrl(kind)).catch((err: unknown) => {
      console.error(`tab ${instanceId} failed to load:`, err);
      this.close(instanceId);
    });
    this.activate(instanceId);
    return this.summaryOf(entry);
  }

  activate(instanceId: string): void {
    if (!this.entries.has(instanceId)) return;
    this.activeId = instanceId;
    for (const entry of this.entries.values()) {
      entry.view.setVisible(entry.instanceId === instanceId);
    }
    this.layout();
    this.broadcast();
  }

  close(instanceId: string): void {
    const entry = this.entries.get(instanceId);
    if (!entry) return;
    this.removeEntry(entry);
    if (!entry.wc.isDestroyed()) entry.wc.close();
  }

  /** 批量关闭；“首选存活者”在 active 被关掉时接管焦点（对照 cherry-studio closeTabs）。 */
  closeMany(ids: readonly string[], activateId?: string): void {
    for (const id of ids) {
      const entry = this.entries.get(id);
      if (!entry) continue;
      this.removeEntry(entry);
      if (!entry.wc.isDestroyed()) entry.wc.close();
    }
    if (activateId !== undefined && this.entries.has(activateId)) {
      this.activate(activateId);
    }
  }

  /** “+” 新建菜单：host 页面 DOM 会被 tab 的原生子视图盖住，必须由主进程弹原生 Menu。 */
  popupNewTabMenu(): void {
    const menu = Menu.buildFromTemplate(
      this.kinds.map((k) => ({
        label: `新建 ${k.title}`,
        click: () => {
          this.create(k.id);
        },
      })),
    );
    menu.popup({ window: this.win });
  }

  /** tab 右键菜单（同上，必须原生 Menu）：关闭 / 关闭其他 / 关闭右侧。 */
  popupTabContextMenu(instanceId: string): void {
    const order = this.summaries();
    const index = order.findIndex((t) => t.instanceId === instanceId);
    if (index < 0) return;
    const others = order.filter((t) => t.instanceId !== instanceId);
    const toRight = order.slice(index + 1);
    const menu = Menu.buildFromTemplate([
      {
        label: "关闭",
        enabled: true,
        click: () => this.close(instanceId),
      },
      {
        label: "关闭其他",
        enabled: others.length > 0,
        click: () =>
          this.closeMany(
            others.map((t) => t.instanceId),
            instanceId,
          ),
      },
      {
        label: "关闭右侧",
        enabled: toRight.length > 0,
        click: () =>
          this.closeMany(
            toRight.map((t) => t.instanceId),
            instanceId,
          ),
      },
    ]);
    menu.popup({ window: this.win });
  }

  /** 由 tab 自己发起的关闭（IPC 用 sender 定位调用者）。 */
  closeByWebContents(wcId: number): void {
    const id = this.byWebContents.get(wcId);
    if (id !== undefined) this.close(id);
  }

  setTitleByWebContents(wcId: number, title: string): void {
    const id = this.byWebContents.get(wcId);
    const entry = id !== undefined ? this.entries.get(id) : undefined;
    if (!entry || title.trim().length === 0) return;
    entry.title = title;
    this.broadcast();
  }

  webContentsOf(instanceId: string): WebContents | undefined {
    return this.entries.get(instanceId)?.wc;
  }

  /** 当前活动 tab 的 webContents（调试菜单用）。 */
  activeWebContents(): WebContents | undefined {
    if (this.activeId === undefined) return undefined;
    return this.entries.get(this.activeId)?.wc;
  }

  /** 所有存活 tab 的 webContents（广播类事件用，如 theme:changed）。 */
  tabWebContents(): WebContents[] {
    return [...this.entries.values()].map((e) => e.wc);
  }

  whoami(wcId: number): Whoami {
    if (wcId === this.win.webContents.id) return { role: "host" };
    const id = this.byWebContents.get(wcId);
    const entry = id !== undefined ? this.entries.get(id) : undefined;
    if (!entry) return { role: "unknown" };
    return { role: "tab", instanceId: entry.instanceId, kind: entry.kind };
  }

  summaries(): TabSummary[] {
    return [...this.entries.values()].map((e) => this.summaryOf(e));
  }

  private summaryOf(entry: TabEntry): TabSummary {
    return {
      instanceId: entry.instanceId,
      kind: entry.kind,
      title: entry.title,
      active: entry.instanceId === this.activeId,
    };
  }

  /** 把整份 tab 状态推给 host 页面（唯一状态源在主进程）。 */
  private broadcast(): void {
    if (this.win.isDestroyed()) return;
    this.win.webContents.send("tabs:changed", this.summaries());
  }

  private layout(): void {
    if (this.win.isDestroyed()) return;
    const [width = 0, height = 0] = this.win.getContentSize();
    const entry =
      this.activeId !== undefined ? this.entries.get(this.activeId) : undefined;
    if (!entry) return;
    entry.view.setBounds({
      x: 0,
      y: TABBAR_HEIGHT,
      width,
      height: Math.max(0, height - TABBAR_HEIGHT),
    });
  }

  /** 幂等清理：close() 与 destroyed 事件都指向这里，只生效一次。 */
  private removeEntry(entry: TabEntry): void {
    if (!this.entries.has(entry.instanceId)) return;
    this.entries.delete(entry.instanceId);
    this.byWebContents.delete(entry.wcId);
    if (!this.win.isDestroyed()) {
      this.win.contentView.removeChildView(entry.view);
    }
    if (this.activeId === entry.instanceId) {
      this.activeId = undefined;
      // 简单策略：激活剩下最新创建的那个
      const rest = [...this.entries.keys()].at(-1);
      if (rest !== undefined) {
        this.activate(rest);
      } else {
        this.broadcast();
      }
    } else {
      this.broadcast();
    }
  }
}

/** 开关某 webContents 的 devtools（F12 与调试菜单共用）。 */
export function toggleDevtools(wc: WebContents): void {
  if (wc.isDevToolsOpened()) {
    wc.closeDevTools();
  } else {
    wc.openDevTools({ mode: "detach" });
  }
}

/**
 * 统一加固每个 webContents（host 与每个 tab）：
 * F12 开关 devtools；拒绝一切导航与 window.open —— 应用只装载本地打包页面。
 */
export function hardenWebContents(wc: WebContents): void {
  wc.on("before-input-event", (_event, input) => {
    if (input.type === "keyDown" && input.key === "F12") {
      toggleDevtools(wc);
    }
  });
  const deny = (event: { preventDefault(): void }) => event.preventDefault();
  wc.on("will-navigate", deny);
  wc.on("will-redirect", deny);
  wc.setWindowOpenHandler(() => ({ action: "deny" }));
}
