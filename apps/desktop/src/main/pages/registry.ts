/**
 * 运行时页面注册表：扫描内置/外部页面目录，管理 main 扩展的 activate/deactivate。
 * - 内置目录（app.getAppPath()/pages）优先；外部目录（userData/pages）同 id 冲突则跳过
 * - 页面 main v1 必须是 CJS：createRequire + 清 require.cache 才能可靠热重载
 * - 页面登记的 IPC 由 tracker 兜底清理；页面自己的 deactivate 负责其余资源
 * - 卸载只做运行时注销，不删除用户目录；内置页面只能重载
 */

import { app, ipcMain, session } from "electron";
import type { BrowserWindow } from "electron";
import { cp, mkdir, readdir, realpath, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";
import type {
  PageIpc,
  PageIpcHandler,
  PageMainContext,
  PageMainModule,
  PageManifest,
  PageSource,
  PageSummary,
} from "@pages/sdk";
import { isInside, readPageManifest } from "./manifest";

export type PageState = "inactive" | "active" | "error";

/** 对外暴露的页面运行时（TabManager / 菜单 / smoke 用） */
export interface PageRuntime {
  readonly manifest: PageManifest;
  readonly root: string;
  readonly source: PageSource;
  readonly state: PageState;
  readonly error?: string;
  readonly entryUrl: string;
  readonly preloadPath?: string;
  readonly mainPath?: string;
}

interface MutablePage extends PageRuntime {
  state: PageState;
  error?: string;
  deactivate?: () => void | Promise<void>;
  tracker?: IpcTracker;
}

interface IpcTracker {
  ipc: PageIpc;
  dispose(): void;
}

export interface PageRegistryOptions {
  builtinDir: string;
  externalDir: string;
  /** 静态 tab kind，页面 id 不能与之冲突 */
  reservedIds: readonly string[];
  getWindow(): BrowserWindow | undefined;
  onDidChange?(): void;
  log?(...args: unknown[]): void;
}

function createIpcTracker(): IpcTracker {
  const handlers: string[] = [];
  const listeners: { channel: string; listener: PageIpcHandler }[] = [];
  return {
    ipc: {
      handle(channel, listener) {
        // PageIpcEvent 是 Electron 事件的最小结构子集，运行时完全相同
        ipcMain.handle(channel, listener as never);
        handlers.push(channel);
      },
      on(channel, listener) {
        ipcMain.on(channel, listener as never);
        listeners.push({ channel, listener });
      },
    },
    dispose() {
      for (const channel of handlers) ipcMain.removeHandler(channel);
      for (const { channel, listener } of listeners) {
        ipcMain.removeListener(channel, listener as never);
      }
      handlers.length = 0;
      listeners.length = 0;
    },
  };
}

const ACTIVATE_TIMEOUT_MS = 8_000;

/** 页面 main 的 activate 可能永不 resolve；超时后降级为 error，不拖死宿主 */
async function withTimeout<T>(
  promise: Promise<T>,
  ms: number,
  message: string,
): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error(message)), ms);
      }),
    ]);
  } finally {
    if (timer !== undefined) clearTimeout(timer);
  }
}

const encodeEntry = (entry: string): string =>
  entry.split(/[\\/]/).filter(Boolean).map(encodeURIComponent).join("/");

export class PageRegistry {
  private readonly pages = new Map<string, MutablePage>();

  private constructor(private readonly opts: PageRegistryOptions) {}

  static async create(opts: PageRegistryOptions): Promise<PageRegistry> {
    const registry = new PageRegistry(opts);
    await registry.scan(opts.builtinDir, "builtin");
    await registry.scan(opts.externalDir, "external");
    return registry;
  }

  list(): readonly PageSummary[] {
    return [...this.pages.values()].map((page) => this.summaryOf(page));
  }

  get(id: string): PageRuntime | undefined {
    return this.pages.get(id);
  }

  rootOf(id: string): string | undefined {
    const page = this.pages.get(id);
    // active / error 都允许加载 renderer；只有尚未激活（inactive）才拒绝
    return page !== undefined && page.state !== "inactive"
      ? page.root
      : undefined;
  }

  async activateAll(): Promise<void> {
    for (const page of this.pages.values()) {
      await this.activateMain(page);
    }
  }

  async install(sourceDir: string): Promise<PageSummary> {
    const sourceRoot = await realpath(sourceDir);
    const manifest = await readPageManifest(sourceRoot);
    if (this.opts.reservedIds.includes(manifest.id)) {
      throw new Error(`页面 id 与静态 tab 冲突：${manifest.id}`);
    }
    if (this.pages.has(manifest.id)) {
      throw new Error(`页面已安装：${manifest.id}`);
    }
    const dest = join(this.opts.externalDir, manifest.id);
    if (
      sourceRoot === dest ||
      isInside(sourceRoot, dest) ||
      isInside(dest, sourceRoot)
    ) {
      throw new Error(`不能从页面安装目录自身安装：${sourceRoot}`);
    }
    if (existsSync(dest)) {
      throw new Error(`目标目录已存在，请先手动处理：${dest}`);
    }

    await mkdir(this.opts.externalDir, { recursive: true });
    await cp(sourceRoot, dest, {
      recursive: true,
      errorOnExist: true,
      force: false,
    });
    try {
      const root = await realpath(dest);
      const copied = await readPageManifest(root);
      if (copied.id !== manifest.id) {
        throw new Error("复制后的 page.json id 与源不一致");
      }
      const page = this.makePage(copied, root, "external");
      this.pages.set(page.manifest.id, page);
      await this.activateMain(page);
      this.changed();
      return this.summaryOf(page);
    } catch (err) {
      this.pages.delete(manifest.id);
      await rm(dest, { recursive: true, force: true }).catch(() => undefined);
      throw err;
    }
  }

  async uninstall(id: string): Promise<void> {
    const page = this.pages.get(id);
    if (page === undefined) throw new Error(`页面不存在：${id}`);
    if (page.source === "builtin") {
      throw new Error("内置页面不能卸载，只能重载");
    }
    await this.deactivate(page);
    this.pages.delete(id);
    this.changed();
  }

  async reload(id: string): Promise<PageSummary> {
    const page = this.pages.get(id);
    if (page === undefined) throw new Error(`页面不存在：${id}`);
    await this.deactivate(page);
    // 资源名带 hash，清掉 HTTP 缓存才能拿到更新后的 bundle
    await session
      .fromPartition(`persist:page-${id}`)
      .clearCache()
      .catch((err: unknown) => this.log("clearCache failed:", err));
    try {
      const manifest = await readPageManifest(page.root);
      if (manifest.id !== id) {
        throw new Error(`重载后 page id 改变（${id} → ${manifest.id}），拒绝`);
      }
      Object.assign(page, this.makePage(manifest, page.root, page.source));
      await this.activateMain(page);
    } catch (err) {
      page.state = "error";
      page.error = err instanceof Error ? err.message : String(err);
      throw err;
    } finally {
      this.changed();
    }
    return this.summaryOf(page);
  }

  async dispose(): Promise<void> {
    for (const page of this.pages.values()) {
      await this.deactivate(page);
    }
  }

  private async scan(dir: string, source: PageSource): Promise<void> {
    const entries = await readdir(dir, { withFileTypes: true }).catch(() => []);
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      try {
        const root = await realpath(join(dir, entry.name));
        const manifest = await readPageManifest(root);
        if (this.opts.reservedIds.includes(manifest.id)) {
          this.log(`跳过 ${manifest.id}：与静态 tab id 冲突`);
          continue;
        }
        if (this.pages.has(manifest.id)) {
          this.log(
            `跳过 ${manifest.id}：id 已被 ${this.pages.get(manifest.id)?.source} 页面占用`,
          );
          continue;
        }
        this.pages.set(manifest.id, this.makePage(manifest, root, source));
      } catch (err) {
        this.log(`跳过非法页面 ${join(dir, entry.name)}：`, err);
      }
    }
  }

  private makePage(
    manifest: PageManifest,
    root: string,
    source: PageSource,
  ): MutablePage {
    return {
      manifest,
      root,
      source,
      state: "inactive",
      error: undefined,
      entryUrl: `page://${manifest.id}/${encodeEntry(manifest.entry)}`,
      preloadPath:
        manifest.preload !== undefined
          ? resolve(root, manifest.preload)
          : undefined,
      mainPath:
        manifest.main !== undefined ? resolve(root, manifest.main) : undefined,
    };
  }

  private async activateMain(page: MutablePage): Promise<void> {
    page.state = "inactive";
    page.error = undefined;
    if (page.mainPath === undefined) {
      page.state = "active";
      return;
    }

    const tracker = createIpcTracker();
    try {
      const dataDir = join(
        app.getPath("userData"),
        "page-data",
        page.manifest.id,
      );
      await mkdir(dataDir, { recursive: true });

      const req = createRequire(page.mainPath);
      const resolved = req.resolve(page.mainPath);
      delete req.cache[resolved];
      const mod = req(resolved) as PageMainModule & {
        default?: PageMainModule;
      };
      const target = typeof mod.activate === "function" ? mod : mod.default;
      if (target === undefined || typeof target.activate !== "function") {
        throw new Error("main.js 未导出 activate()");
      }

      const ctx: PageMainContext = {
        manifest: page.manifest,
        pageRoot: page.root,
        dataDir,
        appPath: app.getAppPath(),
        ipc: tracker.ipc,
        getWindow: () => this.opts.getWindow(),
        log: (...args) =>
          this.opts.log?.(`[page:${page.manifest.id}]`, ...args),
      };
      // 先挂 deactivate：activate 中途抛错/超时时仍能回滚它已占用的资源
      page.deactivate = () => target.deactivate?.();
      await withTimeout(
        Promise.resolve(target.activate(ctx)),
        ACTIVATE_TIMEOUT_MS,
        `main activate timeout (${page.manifest.id})`,
      );
      page.tracker = tracker;
      page.state = "active";
    } catch (err) {
      try {
        await page.deactivate?.();
      } catch (deactivateErr) {
        this.log("deactivate after failed activate:", deactivateErr);
      }
      page.deactivate = undefined;
      tracker.dispose();
      page.state = "error";
      page.error = err instanceof Error ? err.message : String(err);
      this.log("main activate failed:", err);
    }
  }

  private async deactivate(page: MutablePage): Promise<void> {
    try {
      await page.deactivate?.();
    } catch (err) {
      this.log("deactivate failed:", err);
    }
    page.deactivate = undefined;
    page.tracker?.dispose();
    page.tracker = undefined;
    page.state = "inactive";
    page.error = undefined;
  }

  private summaryOf(page: MutablePage): PageSummary {
    return {
      id: page.manifest.id,
      name: page.manifest.name,
      color: page.manifest.color ?? "#565f89",
      source: page.source,
      state: page.state,
      hasMain: page.mainPath !== undefined,
      error: page.error,
    };
  }

  private changed(): void {
    this.opts.onDidChange?.();
  }

  private log(...args: unknown[]): void {
    this.opts.log?.(...args);
  }
}
