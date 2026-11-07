/**
 * resolver/registry —— 平台播放/弹幕能力注册表。
 *
 * 平台实现（client/签名/播放解析/弹幕）已迁入 platform-rss-* 插件包；本基座只持
 * 路由（router.ts）与注册表。插件 bundle（channels.js）在数据服务进程加载时，
 * 由服务侧（channels.registerPlugin）同时登记渠道与本注册表 —— 与 crawler 渠道
 * 注册表同一套「注册动作发生在消费方进程」的契约，避免插件 bundle 内孤立 Map。
 *
 * 未注册平台的能力调用抛「未安装平台插件」可诊断错误（ResolveError）。
 */

import type { Stream } from "@amoeba/xml"

import type { DanmakuStream } from "../danmaku/types.ts"
import type { SourceInfo } from "../index.ts"
import type { ResolvePlatform } from "./router.ts"

/** resolver 解析参数：复用 SourceInfo（含 cookie），必要时扩展。 */
export type ResolveInfo = SourceInfo & { cookie?: string }

export interface DanmakuInfo {
  cookie?: string
  /** 视频 VOD 弹幕（带 timeMs 按时间轴）还是 live 直播聊天。缺省由路由 kind 推断。 */
  kind?: "vod" | "live"
}

/** 章节时间点（秒 + UP 主写的标题；目前只有 bili 有）。 */
export interface PlaybackChapter {
  t: number
  label: string
}

/** 平台播放/弹幕能力（由 platform-rss-* 插件实现并注册）。 */
export interface PlatformPlayback {
  /** 路由平台标识（router.ts 的 ResolvePlatform 取值）。 */
  readonly platform: ResolvePlatform
  /** 视频可播流解析（kind=video 路由的平台）。 */
  resolveVideoPlay?(id: string, info?: ResolveInfo): Promise<{ streams: Stream[]; chapters?: PlaybackChapter[] }>
  /** 直播可播流解析（kind=live 路由的平台）。 */
  resolveLivePlay?(id: string, info?: ResolveInfo): Promise<Stream[]>
  /** 弹幕流（无弹幕能力的平台不实现）。 */
  getDanmaku?(id: string, opts?: DanmakuInfo): DanmakuStream
}

const REGISTRY = new Map<ResolvePlatform, PlatformPlayback>()

/** 注册（覆盖式，与 registerChannel 同语义）。 */
export function registerPlayback(playback: PlatformPlayback): void {
  REGISTRY.set(playback.platform, playback)
}

export function getPlayback(platform: ResolvePlatform): PlatformPlayback | undefined {
  return REGISTRY.get(platform)
}

/** 仅测试用：清空注册表。 */
export function __resetPlayback(): void {
  REGISTRY.clear()
}
