/**
 * TabManager：tab 实例 = 一个 WebContentsView（独立 webContents / preload / 生命周期），
 * 挂在 BrowserWindow 的 contentView 上，叠在 host ui 之上：顶部 44px 留给 tab 条，其余归活动 tab。
 * 静态 tab（编译期子包）与 runtime 页面（page://）共用这套生命周期：
 * - 静态 tab：每次实例一个内存 partition
 * - 页面：persist:page-<id> 分区（localStorage 跨实例/重启保留），preload 来自页面包
 * 所有 tab 状态的唯一权威源在这里，host 页面只做投影（tabs:changed）。
 */

import { Menu, session, WebContentsView, app } from "electron";
import type {
  BrowserWindow,
  MenuItemConstructorOptions,
  WebContents,
} from "electron";
import { join } from "node:path";
import type { TabManifest, TabSummary, Whoami } from "@host/sdk";
import type { PageActions } from "./pages/actions";
import type { PageRegistry, PageRuntime } from "./pages/registry";
import { TAB_KINDS } from "./registry";
import { tabUrl } from "./renderTarget";

/** host 页面里 tab 条的高度；主进程与 ui/index.html 里保持一致。 */
const TABBAR_HEIGHT = 44;

const HOST_PRELOAD = join(app.getAppPath(), "out", "preload", "index.js");
const PAGE_BRIDGE_PRELOAD = join(
  app.getAppPath(),
  "out",
  "preload",
  "pageBridge.js",
);

/** 每个页面 session 只叠加一次宿主 bridge（页面自有 preload 保持不变） */
const bridgedPartitions = new Set<string>();

function ensurePageBridge(partition: string): void {
  if (bridgedPartitions.has(partition)) return;
  session
    .fromPartition(partition)
    .registerPreloadScript({ type: "frame", filePath: PAGE_BRIDGE_PRELOAD });
  bridgedPartitions.add(partition);
}

interface TabEntry {
  instanceId: string;
  kind: string;
  title: string;
  /** 页面 tab 才有：来源页面 id */
  pageId?: string;
  view: WebContentsView;
  /** 创建时固化 webContents 引用与 id：destroyed 之后 view.webContents 不可再访问 */
  wc: WebContents;
  wcId: number;
  /** did-finish-load 前不显示 view，host 在内容区渲染过渡页 */
  loading: boolean;
}

interface MountInput {
  instanceId: string;
  kind: string;
  title: string;
  pageId?: string;
  view: WebContentsView;
}

export class TabManager {
  private readonly win: BrowserWindow;
  private readonly pages: PageRegistry;
  private readonly actions: PageActions;
  private readonly entries = new Map<string, TabEntry>();
  private readonly byWebContents = new Map<number, string>();
  private activeId: string | undefined;
  private seq = 0;

  constructor(win: BrowserWindow, pages: PageRegistry, actions: PageActions) {
    this.win = win;
    this.pages = pages;
    this.actions = actions;
    win.on("resize", () => this.layout());
    // host 重载（Ctrl+R / devtools reload）后重发整份状态，避免 tab 条与主进程脱节
    win.webContents.on("did-finish-load", () => this.broadcast());
  }

  /** 静态 tab + 所有 active 页面，供 host 的圆点颜色与菜单使用 */
  get kinds(): readonly TabManifest[] {
    return [
      ...TAB_KINDS,
      ...this.pages
        .list()
        .filter((page) => page.state === "active")
        .map((page) => ({
          id: page.id,
          title: page.name,
          color: page.color,
        })),
    ];
  }

  /** id 既可以是静态 tab kind，也可以是页面 id */
  create(id: string): TabSummary | undefined {
    const staticManifest = TAB_KINDS.find((m) => m.id === id);
    if (staticManifest !== undefined) {
      const instanceId = `${id}-${++this.seq}`;
      return this.mount(
        {
          instanceId,
          kind: id,
          title: staticManifest.title,
          view: this.createView(HOST_PRELOAD, instanceId),
        },
        tabUrl(id),
      );
    }

    const page = this.pages.get(id);
    // inactive 只在页面激活完成前出现；main 扩展失败（error）仍允许打开 renderer
    if (page === undefined || page.state === "inactive") return undefined;
    return this.createPageTab(page);
  }

  private createPageTab(page: PageRuntime): TabSummary {
    const instanceId = `${page.manifest.id}-${++this.seq}`;
    const partition = `persist:page-${page.manifest.id}`;
    ensurePageBridge(partition);
    return this.mount(
      {
        instanceId,
        kind: page.manifest.id,
        title: page.manifest.name,
        pageId: page.manifest.id,
        view: this.createView(page.preloadPath ?? HOST_PRELOAD, partition),
      },
      page.entryUrl,
    );
  }

  private createView(preload: string, partition: string): WebContentsView {
    return new WebContentsView({
      webPreferences: {
        preload,
        partition,
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
      },
    });
  }

  private mount(input: MountInput, url: string): TabSummary {
    const entry: TabEntry = {
      ...input,
      wc: input.view.webContents,
      wcId: input.view.webContents.id,
      loading: true,
    };
    this.entries.set(entry.instanceId, entry);
    this.byWebContents.set(entry.wcId, entry.instanceId);
    this.win.contentView.addChildView(entry.view);
    hardenWebContents(entry.wc);
    // 页面自己 window.close() 等外部销毁也走同一套清理
    entry.wc.once("destroyed", () => this.removeEntry(entry));
    // 加载完成才显示 view；此前 activate() 保持它不可见，内容区由 host 过渡页占位
    entry.wc.once("did-finish-load", () => {
      entry.loading = false;
      if (this.activeId === entry.instanceId) {
        entry.view.setVisible(true);
      }
      this.broadcast();
    });
    entry.wc.loadURL(url).catch((err: unknown) => {
      console.error(`tab ${entry.instanceId} failed to load:`, err);
      this.close(entry.instanceId);
    });
    this.activate(entry.instanceId);
    return this.summaryOf(entry);
  }

  activate(instanceId: string): void {
    if (!this.entries.has(instanceId)) return;
    this.activeId = instanceId;
    for (const entry of this.entries.values()) {
      entry.view.setVisible(entry.instanceId === instanceId && !entry.loading);
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

  /** 拖拽排序：把 instanceId 移到 overInstanceId 当前的位置（Map 插入序即 tab 顺序）。 */
  move(instanceId: string, overInstanceId: string): void {
    const ids = [...this.entries.keys()];
    const from = ids.indexOf(instanceId);
    const to = ids.indexOf(overInstanceId);
    if (from < 0 || to < 0 || from === to) return;
    const [moved] = ids.splice(from, 1);
    if (moved === undefined) return;
    ids.splice(to, 0, moved);
    const ordered = ids
      .map((id) => this.entries.get(id))
      .filter((entry): entry is TabEntry => entry !== undefined);
    this.entries.clear();
    for (const entry of ordered) this.entries.set(entry.instanceId, entry);
    this.broadcast();
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

  /** 关闭某个页面打开的全部 tab（页面卸载/重载前调用） */
  closeByPage(pageId: string): void {
    const ids = [...this.entries.values()]
      .filter((entry) => entry.pageId === pageId)
      .map((entry) => entry.instanceId);
    this.closeMany(ids);
  }

  /** 页面安装/卸载后通知 host 重新拉取 kinds（圆点颜色） */
  notifyKindsChanged(): void {
    if (this.win.isDestroyed()) return;
    this.win.webContents.send("tabs:kinds-changed");
  }

  /** “+” 新建菜单：host 页面 DOM 会被 tab 的原生子视图盖住，必须由主进程弹原生 Menu。 */
  popupNewTabMenu(): void {
    const pages = this.pages.list();
    const template: MenuItemConstructorOptions[] = [
      ...TAB_KINDS.map((k) => ({
        label: `新建 ${k.title}`,
        click: () => {
          this.create(k.id);
        },
      })),
      ...pages.map((page) => ({
        label: `新建 ${page.name}`,
        enabled: page.state !== "inactive",
        click: () => {
          this.create(page.id);
        },
      })),
      { type: "separator" },
      {
        label: "安装页面…",
        click: () => {
          void this.actions.install();
        },
      },
    ];
    if (pages.length > 0) {
      template.push({
        label: "管理页面",
        submenu: pages.map((page) => ({
          label:
            page.state === "error" ? `${page.name}（扩展错误）` : page.name,
          submenu: [
            {
              label: "重载",
              click: () => {
                void this.actions.reload(page.id);
              },
            },
            ...(page.source === "external"
              ? [
                  {
                    label: "卸载",
                    click: () => {
                      void this.actions.uninstall(page.id);
                    },
                  },
                ]
              : []),
          ],
        })),
      });
    }
    Menu.buildFromTemplate(template).popup({ window: this.win });
  }

  /** tab 右键菜单（同上，必须原生 Menu）：关闭 / 关闭其他 / 关闭右侧；页面 tab 加管理项。 */
  popupTabContextMenu(instanceId: string): void {
    const order = this.summaries();
    const index = order.findIndex((t) => t.instanceId === instanceId);
    if (index < 0) return;
    const entry = this.entries.get(instanceId);
    const others = order.filter((t) => t.instanceId !== instanceId);
    const toRight = order.slice(index + 1);
    const template: MenuItemConstructorOptions[] = [
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
    ];
    const page =
      entry?.pageId !== undefined ? this.pages.get(entry.pageId) : undefined;
    if (page !== undefined) {
      template.push(
        { type: "separator" },
        {
          label: "重载页面",
          click: () => {
            void this.actions.reload(page.manifest.id);
          },
        },
      );
      if (page.source === "external") {
        template.push({
          label: "卸载页面",
          click: () => {
            void this.actions.uninstall(page.manifest.id);
          },
        });
      }
    }
    Menu.buildFromTemplate(template).popup({ window: this.win });
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
      loading: entry.loading,
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
