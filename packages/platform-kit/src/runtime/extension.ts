/**
 * 平台组件入口工厂（本模块是**所有**平台插件的唯一入口）：
 *
 * - `definePlatformExtension`（数据渠道平台）：渠道 + 播放/弹幕直接注册进
 *   api.gateway，不再交给 rss 数据服务进程。注册的操作（namespace = 组件 id
 *   去掉 platform- 前缀）：
 *   - `<ns>.channels` —— 渠道静态描述（key/name/kind/参数表）
 *   - `<ns>.fetch`    —— 抓取：{ channel, info?, cursor?, existing?, passive? } → FetchResult/FetchPage
 *   - `<ns>.play`     —— 播放：{ url | id, kind?, info? } → { id, streams, chapters? }
 *   - `<ns>.danmaku`  —— 弹幕：{ url | id, kind?, cookie? } → { id, kind, subscribe }
 * - `defineMediaPlatformExtension`（媒体元数据平台）：provider 包成
 *   `<source>` 的 search / fetchMetadata / fetchEpisodes（tags: media.metadata）。
 *
 * 两类平台共用同一套运行时底座：appHost 由 electron-host 安装（HTTP/WS/JS/
 * browser），op 失败统一包成 PlatformOpError（平台.操作 上下文 + cause）。
 * 卸载语义（v1）：注册活在网关内，组件停用不撤销，直到网关 reload。
 *
 * 浏览器型渠道（xhs/weibo/douyin）需在 inject 里声明 rss.browser，
 * 由 CDP 代理构造 BrowserBackend。
 */

import type { PageMainContext } from "@host/sdk";
import { PlatformOpError } from "@tabs/platform-contracts";
import type {
  MediaMetadata,
  MediaType,
  MetadataProvider,
} from "@tabs/platform-contracts";

import type { DataChannel, FetchResult, SourceInfo } from "../data";
import { installPlatformAppHost } from "./electron-host";
import {
  matchRoute,
  type DanmakuInfo,
  type PlatformPlayback,
} from "./playback";

/** api-gateway 的服务面（与 packages/api-gateway 的结构一致）。 */
interface ApiGateway {
  register(reg: {
    namespace: string;
    label: string;
    tags?: string[];
    ops: Record<string, { description?: string; invoke: (params: unknown) => unknown }>;
  }): void;
}

/** rss.browser 的服务面（本仓库 rss-browser 组件提供）。 */
interface BrowserService {
  endpoint(): { port: number };
}

export interface PlatformExtensionSpec {
  channels: DataChannel[];
  /** 平台的播放/弹幕能力（无播放能力的平台省略）。 */
  playback?: PlatformPlayback;
}

export interface PlatformExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

/** op 出口的统一错误包装：失败必带 平台.操作 上下文（媒体/数据平台一致）。 */
async function opGuard<T>(
  platform: string,
  op: string,
  fn: () => T | Promise<T>,
): Promise<T> {
  try {
    return await fn();
  } catch (cause) {
    if (cause instanceof PlatformOpError) throw cause;
    throw new PlatformOpError(platform, op, cause);
  }
}

interface ChannelMeta {
  key: string;
  name: string;
  kind: string;
  passive?: boolean;
  sourceInfoTpl?: Array<{
    key: string;
    label: string;
    required?: boolean;
    placeholder?: string;
  }>;
  defaultInfo?: SourceInfo;
}

function channelMeta(c: DataChannel): ChannelMeta {
  return {
    key: c.key,
    name: c.name,
    kind: c.kind,
    ...(c.passive === true ? { passive: true } : {}),
    ...(Array.isArray(c.sourceInfoTpl) ? { sourceInfoTpl: c.sourceInfoTpl } : {}),
    ...(c.defaultInfo !== undefined ? { defaultInfo: c.defaultInfo } : {}),
  };
}

function describe(meta: ChannelMeta[]): ChannelMeta[] {
  return meta;
}

function findChannel(
  channels: Map<string, DataChannel>,
  ns: string,
  key: unknown,
): DataChannel {
  const k = String(key ?? "");
  const channel = channels.get(k);
  if (channel === undefined) {
    throw new Error(`${ns}: 未知渠道 "${k}"`);
  }
  return channel;
}

async function fetchChannel(
  ns: string,
  channels: Map<string, DataChannel>,
  params: unknown,
): Promise<FetchResult | unknown> {
  const p = params as {
    channel?: string;
    info?: SourceInfo;
    cursor?: string;
    /** 消费方已持有条数；滚动型渠道（xhs/douyin）首次翻页必须用它定位起点。 */
    existing?: number;
    passive?: boolean;
  };
  const channel = findChannel(channels, ns, p.channel);
  const source = channel.getSource(p.info ?? {});
  if (p.passive === true) {
    if (source.fetchPassive === undefined) {
      throw new Error(`${ns}: 渠道 ${channel.key} 不支持被动采集`);
    }
    const result = await source.fetchPassive();
    if (result === null) throw new Error(`${ns}: 渠道 ${channel.key} 被动采集无数据`);
    return result;
  }
  if (p.cursor !== undefined) {
    if (source.fetchMore === undefined) {
      throw new Error(`${ns}: 渠道 ${channel.key} 不支持翻页`);
    }
    return source.fetchMore(
      p.cursor,
      typeof p.existing === "number" && p.existing >= 0
        ? { existing: p.existing }
        : undefined,
    );
  }
  return source.fetch();
}

interface PlayParams {
  url?: string;
  id?: string;
  kind?: "video" | "live";
  info?: SourceInfo;
}

interface PlayContext {
  id: string;
  kind: "video" | "live";
}

function playTarget(ns: string, playback: PlatformPlayback, p: PlayParams): PlayContext {
  let id = typeof p.id === "string" ? p.id.trim() : "";
  let kind = p.kind;
  if (id === "" && typeof p.url === "string" && p.url !== "") {
    const route = matchRoute(playback.routes ?? [], p.url);
    if (route === undefined) throw new Error(`${ns}: 无法从 URL 解析播放路由: ${p.url}`);
    id = route.id;
    kind = route.kind;
  }
  if (id === "") throw new Error(`${ns}.play 需要 url 或 id`);
  if (kind === undefined) {
    kind = playback.resolveVideoPlay !== undefined ? "video" : "live";
  }
  return { id, kind };
}

async function play(
  ns: string,
  playback: PlatformPlayback,
  params: unknown,
): Promise<unknown> {
  const p = (params ?? {}) as PlayParams;
  const { id, kind } = playTarget(ns, playback, p);
  if (kind === "live") {
    if (playback.resolveLivePlay === undefined) {
      throw new Error(`${ns}: 未实现直播播放解析`);
    }
    return { id, streams: await playback.resolveLivePlay(id, p.info) };
  }
  if (playback.resolveVideoPlay === undefined) {
    throw new Error(`${ns}: 未实现视频播放解析`);
  }
  const result = await playback.resolveVideoPlay(id, p.info);
  return { id, ...result };
}

interface DanmakuParams {
  url?: string;
  id?: string;
  kind?: "vod" | "live";
  cookie?: string;
}

/**
 * id-only 时不硬填 kind —— 交给平台自己的缺省（bili 的 getDanmaku 缺省 vod）；
 * url 路由命中时按路由 kind 推断（video→vod / live→live）。
 */
function danmakuTarget(
  ns: string,
  playback: PlatformPlayback,
  p: DanmakuParams,
): { id: string; kind?: "vod" | "live" } {
  let id = typeof p.id === "string" ? p.id.trim() : "";
  let kind = p.kind;
  if (id === "" && typeof p.url === "string" && p.url !== "") {
    const route = matchRoute(playback.routes ?? [], p.url);
    if (route === undefined) throw new Error(`${ns}: 无法从 URL 解析弹幕路由: ${p.url}`);
    id = route.id;
    kind ??= route.kind === "video" ? "vod" : "live";
  }
  if (id === "") throw new Error(`${ns}.danmaku 需要 url 或 id`);
  return { id, ...(kind !== undefined ? { kind } : {}) };
}

function danmaku(
  ns: string,
  playback: PlatformPlayback,
  params: unknown,
): unknown {
  const p = (params ?? {}) as DanmakuParams;
  if (playback.getDanmaku === undefined) {
    throw new Error(`${ns}: 未实现弹幕能力`);
  }
  const { id, kind } = danmakuTarget(ns, playback, p);
  const info: DanmakuInfo = {
    ...(kind !== undefined ? { kind } : {}),
    ...(p.cookie !== undefined ? { cookie: p.cookie } : {}),
  };
  const stream = playback.getDanmaku(id, info);
  return { id, ...(kind !== undefined ? { kind } : {}), subscribe: stream };
}

/** 平台组件 main 的唯一入口。 */
export function definePlatformExtension(
  spec: PlatformExtensionSpec,
): PlatformExtensionModule {
  return {
    async activate(ctx) {
      installPlatformAppHost();

      // 浏览器型渠道：组件 inject 了 rss.browser 时构造 CDP BrowserBackend。
      if ((ctx.manifest.inject ?? []).includes("rss.browser")) {
        const svc = ctx.get("rss.browser") as BrowserService | undefined;
        if (svc !== undefined) {
          const { createCdpPortBrowser } = await import(
            "./browser/node/browser-cdp.ts"
          );
          installPlatformAppHost({ browser: createCdpPortBrowser({ port: svc.endpoint().port }) });
        }
      }

      const gw = ctx.require<ApiGateway>("api.gateway");
      const namespace = ctx.manifest.id.replace(/^platform-/, "");
      const byKey = new Map(spec.channels.map((c) => [c.key, c] as const));
      const meta = spec.channels.map(channelMeta);

      const ops: Record<
        string,
        { description?: string; invoke: (params: unknown) => unknown }
      > = {
        channels: {
          description: "渠道清单（静态描述：key / 名称 / kind / 参数表）",
          invoke: () => describe(meta),
        },
        fetch: {
          description:
            "抓取渠道数据；cursor 翻页（existing=消费方已持有条数，滚动型渠道必需），passive=true 走页面被动采集",
          invoke: (params) =>
            opGuard(namespace, "fetch", () =>
              fetchChannel(namespace, byKey, params),
            ),
        },
      };

      if (spec.playback !== undefined) {
        const playback = spec.playback;
        ops.play = {
          description: "按 url 或 id 解析可播流（视频/直播）",
          invoke: (params) =>
            opGuard(namespace, "play", () => play(namespace, playback, params)),
        };
        if (playback.getDanmaku !== undefined) {
          ops.danmaku = {
            description: "按 url 或 id 获取弹幕流（subscribe 返回取消函数）",
            invoke: (params) =>
              opGuard(namespace, "danmaku", () =>
                danmaku(namespace, playback, params),
              ),
          };
        }
      }

      gw.register({
        namespace,
        label: ctx.manifest.name,
        tags: ["platform.data"],
        ops,
      });
      ctx.log(
        `platform api registered: ${namespace} (${Object.keys(ops).join(", ")})`,
      );
    },
    deactivate() {
      // 注册表在网关内；停用不撤销（v1），随网关 reload 重建。
    },
  };
}

// ── 媒体元数据平台（douban / bangumi / tmdb）─────────────────────────────────

export interface MediaPlatformSpec {
  /** 渠道身份（ScrapeSource，如 "tmdb"）；API 命名空间 = 平台 id 本身 */
  readonly source: string;
  /** 人读标签（如 "TMDB"） */
  readonly label: string;
  /** provider 工厂（独立形态经包根导出复用）；契约见 @tabs/platform-contracts */
  readonly createProvider: () => MetadataProvider;
}

/** 媒体元数据平台插件：provider 包成 <source> API 注册进网关（平台即命名空间）。 */
export function defineMediaPlatformExtension(spec: MediaPlatformSpec): PlatformExtensionModule {
  return {
    async activate(ctx) {
      // 与数据渠道平台共用同一 appHost（HTTP 统一传输面，未来接治理）。
      installPlatformAppHost();

      const gw = ctx.require<ApiGateway>("api.gateway");
      const provider = spec.createProvider();
      gw.register({
        namespace: spec.source,
        label: spec.label,
        tags: ["media.metadata"],
        ops: {
          search: {
            description: "按关键词搜索条目",
            invoke: (params) =>
              opGuard(spec.source, "search", () => {
                const p = params as { keyword?: string; mediaType?: MediaType };
                return provider.search(String(p.keyword ?? ""), p.mediaType ?? "tv");
              }),
          },
          fetchMetadata: {
            description: "按平台条目 id 取完整元数据",
            invoke: (params) =>
              opGuard(spec.source, "fetchMetadata", () => {
                const p = params as { id?: string; mediaType?: MediaType };
                return provider.fetchMetadata(String(p.id ?? ""), p.mediaType ?? "tv");
              }),
          },
          fetchEpisodes: {
            description: "取剧集每集元数据（不支持的平台返回空数组）",
            invoke: (params) =>
              opGuard(spec.source, "fetchEpisodes", () => {
                const p = params as { meta?: unknown; id?: string };
                return provider.fetchEpisodes(p.meta as MediaMetadata, String(p.id ?? ""));
              }),
          },
        },
      });
      ctx.log(`media api registered: ${spec.source}`);
    },
    deactivate() {
      // 注册表在网关内；停用不撤销（v1），随网关 reload 重建
    },
  };
}
