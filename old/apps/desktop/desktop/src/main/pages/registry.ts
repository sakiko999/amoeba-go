/**
 * 运行时组件注册表：扫描内置/外部组件目录，管理 main 扩展的 activate/deactivate。
 * - 组件两类：kind=page（前端子页面，entry 必填，可开 tab）与 kind=service（纯后端服务）
 * - 内置目录（app.getAppPath()/pages）优先；外部目录（userData/pages）同 id 冲突则跳过
 * - 组件 main v1 必须是 CJS：createRequire + 清 require.cache 才能可靠热重载
 * - 组件登记的 IPC 由 tracker 兜底清理；组件自己的 deactivate 负责其余资源
 * - 服务表：ctx.provide 注册（须与 manifest.provides 一致），激活按 provides/inject 拓扑排序，
 *   reload/uninstall 服务组件时级联停用依赖它的组件
 * - 卸载只做运行时注销，不删除用户目录；内置组件只能重载
 */

import { app, ipcMain, session } from "electron";
import type { BrowserWindow } from "electron";
import { cp, mkdir, readdir, realpath, rm } from "node:fs/promises";
import { existsSync } from "node:fs";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";
import type {
  ComponentInfo,
  PageIpc,
  PageIpcHandler,
  PageMainContext,
  PageMainModule,
  PageManifest,
  PageSource,
  PageSummary,
  ProvideOptions,
} from "@host/sdk";
import { isInside, readPageManifest } from "./manifest";
import {
  activationOrder,
  dependentsOf,
  projectDeps,
  ServiceTable,
} from "./services";

export type PageState = "inactive" | "active" | "error";

/** 对外暴露的组件运行时（TabManager / 菜单 / smoke 用） */
export interface PageRuntime {
  readonly manifest: PageManifest;
  readonly root: string;
  readonly source: PageSource;
  readonly state: PageState;
  readonly error?: string;
  /** 仅 kind=page：page:// 入口 URL */
  readonly entryUrl?: string;
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
  getWindow(): BrowserWindow | undefined;
  /** 级联停用依赖方前关闭其 tab（service reload/uninstall 时调用） */
  closeTabs?(pageId: string): void;
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
      removeHandler(channel) {
        const i = handlers.indexOf(channel);
        if (i >= 0) handlers.splice(i, 1);
        ipcMain.removeHandler(channel);
      },
      removeListener(channel, listener) {
        const i = listeners.findIndex(
          (l) => l.channel === channel && l.listener === listener,
        );
        if (i >= 0) listeners.splice(i, 1);
        ipcMain.removeListener(channel, listener as never);
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
  private readonly services = new ServiceTable();

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

  /** 组件依赖视图：状态 + manifest 的 provides/inject（只读，检查页 IPC 用）。 */
  inspect(): ComponentInfo[] {
    return [...this.pages.values()].map((page) => ({
      ...this.summaryOf(page),
      ...(page.manifest.version !== undefined
        ? { version: page.manifest.version }
        : {}),
      provides: page.manifest.provides ?? [],
      inject: page.manifest.inject ?? [],
    }));
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

  /** 按 provides/inject 拓扑激活全部组件（service 先于依赖它的页面） */
  async activateAll(): Promise<void> {
    const order = activationOrder(
      [...this.pages.values()].map((p) => projectDeps(p.manifest)),
    );
    for (const id of order) {
      const page = this.pages.get(id);
      if (page !== undefined) await this.activateMain(page);
    }
  }

  async install(sourceDir: string): Promise<PageSummary> {
    const sourceRoot = await realpath(sourceDir);
    const manifest = await readPageManifest(sourceRoot);
    if (this.pages.has(manifest.id)) {
      throw new Error(`组件已安装：${manifest.id}`);
    }
    const dest = join(this.opts.externalDir, manifest.id);
    if (
      sourceRoot === dest ||
      isInside(sourceRoot, dest) ||
      isInside(dest, sourceRoot)
    ) {
      throw new Error(`不能从组件安装目录自身安装：${sourceRoot}`);
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
        throw new Error("复制后的 manifest id 与源不一致");
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
    if (page === undefined) throw new Error(`组件不存在：${id}`);
    if (page.source === "builtin") {
      throw new Error("内置组件不能卸载，只能重载");
    }
    await this.cascade(id, () => this.deactivate(page));
    this.pages.delete(id);
    this.changed();
  }

  async reload(id: string): Promise<PageSummary> {
    const page = this.pages.get(id);
    if (page === undefined) throw new Error(`组件不存在：${id}`);
    await this.cascade(id, () => this.reloadSelf(page));
    return this.summaryOf(page);
  }

  /**
   * reload/uninstall 的级联包装：先停用依赖本组件服务的组件（关其 tab），
   * 再执行自身操作；完成后重新激活依赖方——reload 后拿到新服务实例，
   * uninstall 后因服务缺失进入 error 态（可诊断，好过隐式 inactive）。
   */
  private async cascade(id: string, op: () => Promise<void>): Promise<void> {
    const deps = this.dependentsOf(id);
    for (const depId of deps) {
      const dep = this.pages.get(depId);
      if (dep === undefined) continue;
      this.opts.closeTabs?.(depId);
      await this.deactivate(dep);
    }
    try {
      await op();
    } finally {
      for (const depId of deps) {
        const dep = this.pages.get(depId);
        if (dep !== undefined) await this.activateMain(dep);
      }
      this.changed();
    }
  }

  /** 依赖本组件 provides 服务（传递闭包不展开，v1 只做一层直接依赖）的其他组件 */
  private dependentsOf(id: string): string[] {
    const page = this.pages.get(id);
    if (page === undefined) return [];
    const provides = (page.manifest.provides ?? []).map((s) => s.name);
    if (provides.length === 0) return [];
    return dependentsOf(
      provides,
      [...this.pages.values()].map((p) => projectDeps(p.manifest)),
    );
  }

  private async reloadSelf(page: MutablePage): Promise<void> {
    await this.deactivate(page);
    if ((page.manifest.kind ?? "page") === "page") {
      // 资源名带 hash，清掉 HTTP 缓存才能拿到更新后的 bundle
      await session
        .fromPartition(`persist:page-${page.manifest.id}`)
        .clearCache()
        .catch((err: unknown) => this.log("clearCache failed:", err));
    }
    try {
      const manifest = await readPageManifest(page.root);
      if (manifest.id !== page.manifest.id) {
        throw new Error(
          `重载后组件 id 改变（${page.manifest.id} → ${manifest.id}），拒绝`,
        );
      }
      Object.assign(page, this.makePage(manifest, page.root, page.source));
      await this.activateMain(page);
    } catch (err) {
      page.state = "error";
      page.error = err instanceof Error ? err.message : String(err);
      throw err;
    }
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
      // out/pages 的共享 npm 依赖层（sync-pages 合并产物），不是组件
      if (entry.name === "node_modules") continue;
      try {
        const root = await realpath(join(dir, entry.name));
        const manifest = await readPageManifest(root);
        if (this.pages.has(manifest.id)) {
          this.log(
            `跳过 ${manifest.id}：id 已被 ${this.pages.get(manifest.id)?.source} 组件占用`,
          );
          continue;
        }
        this.pages.set(manifest.id, this.makePage(manifest, root, source));
      } catch (err) {
        this.log(`跳过非法组件 ${join(dir, entry.name)}：`, err);
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
      entryUrl:
        manifest.kind === "service"
          ? undefined
          : `page://${manifest.id}/${encodeEntry(manifest.entry)}`,
      preloadPath:
        manifest.kind !== "service" && manifest.preload !== undefined
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

      const declared = page.manifest.provides ?? [];
      const inject = page.manifest.inject ?? [];
      const id = page.manifest.id;
      const assertInjected = (name: string, method: string): void => {
        if (!inject.includes(name)) {
          throw new Error(
            `${method}("${name}") 失败：manifest.inject 未声明该服务`,
          );
        }
      };
      const ctx: PageMainContext = {
        manifest: page.manifest,
        pageRoot: page.root,
        dataDir,
        appPath: app.getAppPath(),
        ipc: tracker.ipc,
        getWindow: () => this.opts.getWindow(),
        log: (...args) => this.opts.log?.(`[page:${id}]`, ...args),
        provide: (name, impl, options?: ProvideOptions) => {
          const decl = declared.find((d) => d.name === name);
          if (decl === undefined) {
            throw new Error(
              `provide("${name}") 失败：manifest.provides 未声明该服务`,
            );
          }
          const same = (a?: string[], b?: string[]) =>
            JSON.stringify(a ?? []) === JSON.stringify(b ?? []);
          if (!same(decl.expose, options?.expose)) {
            throw new Error(
              `provide("${name}") 的 expose 与 manifest.provides 声明不一致`,
            );
          }
          this.services.register(name, {
            providerId: id,
            impl,
            expose: decl.expose,
          });
        },
        get: (name) => {
          assertInjected(name, "get");
          return this.services.get(name)?.impl;
        },
        require: <T>(name: string): T => {
          assertInjected(name, "require");
          const entry = this.services.get(name);
          if (entry === undefined) {
            throw new Error(`服务未就绪：${name}（提供方未激活或未安装）`);
          }
          return entry.impl as T;
        },
      };
      // 先挂 deactivate：activate 中途抛错/超时时仍能回滚它已占用的资源
      page.deactivate = () => target.deactivate?.();
      await withTimeout(
        Promise.resolve(target.activate(ctx)),
        ACTIVATE_TIMEOUT_MS,
        `main activate timeout (${page.manifest.id})`,
      );
      // 声明了 provides 却没有全部注册 = 契约违背（依赖方会等到一个空服务）
      for (const decl of declared) {
        if (this.services.get(decl.name)?.providerId !== id) {
          throw new Error(`activate 返回但未 provide 声明的服务：${decl.name}`);
        }
      }
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
      this.services.unregisterByProvider(page.manifest.id);
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
    this.services.unregisterByProvider(page.manifest.id);
    page.state = "inactive";
    page.error = undefined;
  }

  private summaryOf(page: MutablePage): PageSummary {
    return {
      id: page.manifest.id,
      name: page.manifest.name,
      color:
        page.manifest.kind === "service"
          ? "#565f89"
          : (page.manifest.color ?? "#565f89"),
      source: page.source,
      kind: page.manifest.kind ?? "page",
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
