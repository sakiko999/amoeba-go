/**
 * 媒体能力服务：纯能力对象（无 IPC 绑定、无窗口创建），三种宿主形态共用：
 * - 独立应用（backend.ts：createMediaService + bindMediaIpc 到 ipcMain）
 * - 宿主页面扩展（pageEntry.ts：ctx.require 取服务组件实例，bindMediaIpc 到 ctx.ipc）
 * - 宿主服务组件（serviceEntry.ts：createMediaService 后 ctx.provide 注册）
 * dispose 只关 sqlite 缓存；IPC 的注册/释放由各形态的绑定层负责。
 */

import { dialog, shell } from "electron";
import type { BrowserWindow } from "electron";
import { readFile } from "node:fs/promises";
import { isAbsolute, relative } from "node:path";
import type { MediaMetadata } from "../shared/model";
import type {
  LibraryItem,
  OrganizePlan,
  PlanItem,
  ScrapePreview,
  ScrapeResult,
  ScrapeSource,
  SearchResult,
  ShowDetail,
} from "../shared/types";
import { cacheClear, closeCache, configureCache } from "./cache";
import { scanLibrary, showDetail } from "./library";
import {
  metadataForDir,
  readMetadataNfo,
  writeMetadataNfoFile,
} from "./nfoFile";
import { applyOrganize, planOrganize } from "./organize";
import { scrapeApply, scrapePreview, searchMetadata } from "./scrape";

export interface MediaServiceOptions {
  getWindow(): BrowserWindow | undefined;
  /** 元数据缓存目录（默认 userData） */
  dataDir?: string;
}

export interface MediaService {
  /** 让用户选择库根目录；窗口不可用时返回 null */
  pickLibrary(): Promise<string | null>;
  /** 扫描库根（一级子目录 = 作品） */
  scanLibrary(root: string): Promise<LibraryItem[]>;
  /** 单个作品的详情（含 NFO 元数据聚合） */
  showDetail(dir: string): Promise<ShowDetail | null>;
  /** 读图片为 data URL（CSP 不放行 file://）；失败返回 null */
  imageToDataUrl(path: string): Promise<string | null>;
  /** 在资源管理器中显示 */
  reveal(path: string): void;
  /** 元数据搜索（tmdb / bangumi / douban） */
  searchMetadata(
    source: ScrapeSource,
    keyword: string,
    mediaType: "tv" | "movie",
  ): Promise<SearchResult[]>;
  scrapePreview(
    source: ScrapeSource,
    id: string,
    mediaType: "tv" | "movie",
  ): Promise<ScrapePreview>;
  scrapeApply(
    dir: string,
    source: ScrapeSource,
    id: string,
    mediaType: "tv" | "movie",
  ): Promise<ScrapeResult>;
  readNfo(dir: string): Promise<MediaMetadata | undefined>;
  writeNfo(dir: string, metadata: MediaMetadata): Promise<string>;
  /** 整理计划（dry-run）；只允许库根一级子目录 */
  organizePlan(root: string, dir: string): Promise<OrganizePlan>;
  /** 执行整理：永远基于当下磁盘状态重新生成计划（安全铁律） */
  organizeApply(root: string, dir: string): Promise<PlanItem[]>;
  cacheClear(): void;
  /** 关闭 sqlite 缓存（停用/重载时调用） */
  dispose(): void;
}

/* ---------- 参数校验（服务层兜底：未来可能被非 IPC 调用方直接消费） ---------- */

function requireString(value: unknown, name: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${name} is required`);
  }
  return value;
}

function requireMediaType(value: unknown): "tv" | "movie" {
  if (value !== "tv" && value !== "movie") throw new Error("invalid mediaType");
  return value;
}

function requireSource(value: unknown): ScrapeSource {
  if (value !== "tmdb" && value !== "bangumi" && value !== "douban") {
    throw new Error("invalid scrape source");
  }
  return value;
}

function requireMetadata(value: unknown): MediaMetadata {
  const m = value as Partial<MediaMetadata> | null;
  if (
    m === null ||
    typeof m !== "object" ||
    typeof m.title !== "string" ||
    m.title.trim() === ""
  ) {
    throw new Error("invalid metadata");
  }
  if (m.mediaType !== "tv" && m.mediaType !== "movie") {
    throw new Error("invalid metadata mediaType");
  }
  return m as MediaMetadata;
}

/** organize 只允许作用于库根的一级子目录，避免能力被拿来做任意路径移动 */
function requireDirectChild(root: string, dir: string): void {
  const rel = relative(root, dir);
  const parts = rel.split(/[\\/]/).filter(Boolean);
  if (parts.length !== 1 || rel.startsWith("..") || isAbsolute(rel)) {
    throw new Error(`目录必须是库根的一级子目录：${dir}`);
  }
}

const IMAGE_MIME: Record<string, string> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".webp": "image/webp",
};

export function createMediaService(opts: MediaServiceOptions): MediaService {
  configureCache(opts.dataDir);

  return {
    async pickLibrary() {
      const win = opts.getWindow();
      if (win === undefined || win.isDestroyed()) return null;
      const result = await dialog.showOpenDialog(win, {
        properties: ["openDirectory"],
      });
      return result.canceled ? null : (result.filePaths[0] ?? null);
    },

    async scanLibrary(root) {
      const target = requireString(root, "root");
      const started = Date.now();
      const items = await scanLibrary(target);
      const ms = Date.now() - started;
      if (ms > 100) {
        console.log(
          `library:scan ${items.length} items in ${ms}ms (${target})`,
        );
      }
      return items;
    },

    async showDetail(dir) {
      return showDetail(requireString(dir, "dir"));
    },

    async imageToDataUrl(path) {
      const file = requireString(path, "path");
      const mime = IMAGE_MIME[file.slice(file.lastIndexOf(".")).toLowerCase()];
      if (!mime) return null;
      try {
        const buf = await readFile(file);
        return `data:${mime};base64,${buf.toString("base64")}`;
      } catch (err) {
        // 读取失败不静默吞（旧项目教训）
        console.error(`read image failed: ${file}`, err);
        return null;
      }
    },

    reveal(path) {
      if (typeof path === "string" && path.length > 0) {
        void shell.showItemInFolder(path);
      }
    },

    async searchMetadata(source, keyword, mediaType) {
      return searchMetadata(
        requireSource(source),
        requireString(keyword, "keyword"),
        requireMediaType(mediaType),
      );
    },

    async scrapePreview(source, id, mediaType) {
      return scrapePreview(
        requireSource(source),
        requireString(id, "id"),
        requireMediaType(mediaType),
      );
    },

    async scrapeApply(dir, source, id, mediaType) {
      return scrapeApply(
        requireString(dir, "dir"),
        requireSource(source),
        requireString(id, "id"),
        requireMediaType(mediaType),
      );
    },

    readNfo(dir) {
      return readMetadataNfo(requireString(dir, "dir")) ?? null;
    },

    async writeNfo(dir, metadata) {
      return writeMetadataNfoFile(
        requireString(dir, "dir"),
        requireMetadata(metadata),
      );
    },

    async organizePlan(root, dir) {
      const libraryRoot = requireString(root, "root");
      const target = requireString(dir, "dir");
      requireDirectChild(libraryRoot, target);
      return planOrganize(target, libraryRoot, await metadataForDir(target));
    },

    async organizeApply(root, dir) {
      const libraryRoot = requireString(root, "root");
      const target = requireString(dir, "dir");
      requireDirectChild(libraryRoot, target);
      // 重新生成计划：apply 永远基于当下的磁盘状态，不信任前端传来的旧计划
      const meta = await metadataForDir(target);
      const plan = await planOrganize(target, libraryRoot, meta);
      return applyOrganize(plan, meta);
    },

    cacheClear() {
      cacheClear();
    },

    dispose() {
      closeCache();
    },
  };
}
