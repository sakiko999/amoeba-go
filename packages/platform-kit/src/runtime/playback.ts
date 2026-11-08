/**
 * 平台播放/弹幕契约（纯类型 + 一个匹配助手，无注册表）。
 *
 * gateway 模型下调用是带命名空间的（`twitch.play`），URL → id/kind 的路由
 * 知识由各平台在 playback.routes 里自带，平台包不再依赖一张全局路由表。
 */

import type { Stream } from "@tabs/platform-contracts";

import type { DanmakuStream } from "./danmaku/types.ts";
import type { SourceInfo } from "../data.ts";

/** resolver 解析参数：复用 SourceInfo（含 cookie），必要时扩展。 */
export type ResolveInfo = SourceInfo & { cookie?: string };

export interface DanmakuInfo {
  cookie?: string;
  /** 视频 VOD 弹幕（带 timeMs 按时间轴）还是 live 直播聊天。缺省由路由 kind 推断。 */
  kind?: "vod" | "live";
}

/** 章节时间点（秒 + UP 主写的标题；目前只有 bili 有）。 */
export interface PlaybackChapter {
  t: number;
  label: string;
}

/** 平台自带的 URL 路由：URL 形态 → 条目 id + 播放种类。 */
export interface PlatformRoute {
  /** 匹配 item.url 的形态（正则 test）。 */
  test: RegExp;
  kind: "video" | "live";
  /** 从 url 提取条目 id（视频 bvid/直播 roomId 等）；返回 null 视为不命中。 */
  extractId(url: string): string | null;
}

/** 按 url 在路由表里找 { id, kind }；未命中返回 undefined。 */
export function matchRoute(
  routes: readonly PlatformRoute[],
  url: string,
): { id: string; kind: "video" | "live" } | undefined {
  for (const route of routes) {
    if (!route.test.test(url)) continue;
    const id = route.extractId(url);
    if (id !== null && id !== "") return { id, kind: route.kind };
  }
  return undefined;
}

/** 平台播放/弹幕能力（由 platform-* 插件实现，注册进 api.gateway）。 */
export interface PlatformPlayback {
  /** 平台自述 id（日志/诊断用；如 "bilibili" / "huya"）。 */
  readonly platform: string;
  /** 本平台的 URL 路由（play/danmaku 的 url 参数据此取 id/kind）。 */
  readonly routes?: readonly PlatformRoute[];
  /** 视频可播流解析。 */
  resolveVideoPlay?(
    id: string,
    info?: ResolveInfo,
  ): Promise<{ streams: Stream[]; chapters?: PlaybackChapter[] }>;
  /** 直播可播流解析。 */
  resolveLivePlay?(id: string, info?: ResolveInfo): Promise<Stream[]>;
  /** 弹幕流（无弹幕能力的平台不实现）。 */
  getDanmaku?(id: string, opts?: DanmakuInfo): DanmakuStream;
}
