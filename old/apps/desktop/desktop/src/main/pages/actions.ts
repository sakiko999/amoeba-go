/**
 * 组件的用户动作：安装 / 重载 / 卸载（page 与 service 两类组件共用）。
 * 安装会读取 manifest 并弹出明确的信任警告（main.js = 完整主进程权限；
 * service 组件额外提示无界面、开机自启）；卸载/重载先关闭受影响组件已打开
 * 的 tab（含依赖其服务的页面），再通知注册表。
 */

import { dialog } from "electron";
import type { BrowserWindow } from "electron";
import type { PageManifest, PageSummary } from "@host/sdk";
import { readPageManifest } from "./manifest";
import type { PageRegistry } from "./registry";

export interface PageActions {
  install(): Promise<void>;
  reload(id: string): Promise<void>;
  uninstall(id: string): Promise<void>;
  list(): readonly PageSummary[];
}

const messageOf = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

/** 按 manifest 内容生成安装警告的 detail（暴露面在安装期可见） */
function trustDetail(manifest: PageManifest): string {
  const provides = manifest.provides ?? [];
  const inject = manifest.inject ?? [];
  const services = provides
    .map(
      (s) =>
        `\n  • ${s.name}${s.expose ? `（方法：${s.expose.join(", ")}）` : ""}`,
    )
    .join("");
  const deps = inject.length > 0 ? `\n依赖服务：${inject.join(", ")}` : "";
  if (manifest.kind === "service") {
    return (
      `该组件是无界面的后端服务，将以完整 Node.js 权限在宿主主进程中执行，` +
      `并在宿主启动时自动激活。\n提供服务：${services.length > 0 ? services : "（未声明）"}${deps}` +
      `\n\n只安装你信任的来源。组件会被复制到用户数据目录。`
    );
  }
  const trust =
    manifest.main !== undefined
      ? "该页面包含 main.js，将以完整 Node.js 权限在宿主主进程中执行。"
      : "该页面包含自己的 renderer / preload 代码，将在宿主中运行。";
  return (
    `${trust}${services.length > 0 ? `\n同时提供服务：${services}` : ""}${deps}` +
    `\n\n只安装你信任的来源。页面会被复制到用户数据目录。`
  );
}

/** service 组件没有可打开的 tab；reload 后页面组件才重新打开 */
const openable = (summary: PageSummary): boolean => summary.kind === "page";

export function createPageActions(opts: {
  pages: PageRegistry;
  getWindow(): BrowserWindow | undefined;
  closeTabs(pageId: string): void;
  openPage(pageId: string): void;
  onChanged(): void;
}): PageActions {
  const fail = (err: unknown): void => {
    dialog.showErrorBox("组件操作失败", messageOf(err));
  };
  const window = (): BrowserWindow | undefined => {
    const win = opts.getWindow();
    return win !== undefined && !win.isDestroyed() ? win : undefined;
  };

  return {
    list: () => opts.pages.list(),

    async install(): Promise<void> {
      const win = window();
      if (win === undefined) return;
      try {
        const picked = await dialog.showOpenDialog(win, {
          title: "选择组件包目录（含 component.json 或 page.json）",
          properties: ["openDirectory"],
        });
        const dir = picked.filePaths[0];
        if (picked.canceled || dir === undefined) return;

        const manifest = await readPageManifest(dir);
        const kindLabel = manifest.kind === "service" ? "服务组件" : "页面";
        const choice = await dialog.showMessageBox(win, {
          type: "warning",
          title: `安装${kindLabel}`,
          message: `安装「${manifest.name}」（${manifest.id}）？`,
          detail: trustDetail(manifest),
          buttons: ["取消", "安装"],
          defaultId: 0,
          cancelId: 0,
        });
        if (choice.response !== 1) return;

        const summary = await opts.pages.install(dir);
        if (openable(summary)) opts.openPage(summary.id);
        opts.onChanged();
        if (summary.state === "error") {
          await dialog.showMessageBox(win, {
            type: "warning",
            title: "组件已安装，但主进程扩展加载失败",
            message: `「${summary.name}」的 renderer 仍可打开，但其 main.js 未能激活。`,
            detail: summary.error ?? "未知错误",
            buttons: ["知道了"],
          });
        }
      } catch (err) {
        fail(err);
      }
    },

    async reload(id: string): Promise<void> {
      const win = window();
      if (win === undefined) return;
      try {
        opts.closeTabs(id);
        const summary = await opts.pages.reload(id);
        if (openable(summary)) opts.openPage(id);
        opts.onChanged();
        if (summary.state === "error") {
          await dialog.showMessageBox(win, {
            type: "warning",
            title: "组件已重载，但主进程扩展加载失败",
            message: `「${summary.name}」的 renderer 仍可打开，但其 main.js 未能激活。`,
            detail: summary.error ?? "未知错误",
            buttons: ["知道了"],
          });
        }
      } catch (err) {
        fail(err);
      }
    },

    async uninstall(id: string): Promise<void> {
      const win = window();
      if (win === undefined) return;
      const page = opts.pages.get(id);
      if (page === undefined) return;
      const isService = page.manifest.kind === "service";
      try {
        const choice = await dialog.showMessageBox(win, {
          type: "warning",
          title: isService ? "卸载服务组件" : "卸载页面",
          message: `卸载「${page.manifest.name}」？`,
          detail: isService
            ? "将停用该服务并级联停用依赖它的页面（其 tab 会关闭）。" +
              "组件文件保留在用户数据目录；重启宿主后会重新扫描到它。" +
              "彻底移除请手动删除对应目录。"
            : "将关闭该页面的 tab 并停用其主进程扩展。页面文件保留在用户数据目录；" +
              "重启宿主后会重新扫描到它。彻底移除请手动删除对应目录。",
          buttons: ["取消", "卸载"],
          defaultId: 0,
          cancelId: 0,
        });
        if (choice.response !== 1) return;

        opts.closeTabs(id);
        await opts.pages.uninstall(id);
        opts.onChanged();
      } catch (err) {
        fail(err);
      }
    },
  };
}
