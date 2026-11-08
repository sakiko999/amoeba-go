/**
 * 平台组件入口工厂（所有平台插件的唯一入口）与两种现成协议：
 *
 * - `definePlatformExtension({ tags, ops })`：把平台操作注册进 api.gateway；
 *   每个 op 的失败都统一包成 PlatformOpError（平台.操作 + cause）。
 * - `feedOps({ channels, playback })`：内容渠道协议（channels / fetch / play /
 *   danmaku），供 feed/直播类消费方统一调用。
 * - `metadataOps(provider)`：媒体元数据协议（search / fetchMetadata /
 *   fetchEpisodes），供 media-library 统一调用。
 *
 * 协议是消费方约定，不是平台定义：平台可以只实现协议、只注册私有 op，
 * 或在协议结果上追加私有 op。
 *
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

/** 一个平台操作：params 由调用方给出，返回值/异常原样透出（工厂统一包装错误）。 */
export interface PlatformOp {
  description?: string;
  invoke: (params: unknown) => unknown;
}

export type PlatformOps = Record<string, PlatformOp>;

export interface PlatformExtensionSpec {
  /** 能力/协议标签（消费方按 tags 发现平台，如 media.metadata）；缺省无标签。 */
  tags?: string[];
  /** 操作表；函数形式在 activate 时构建（provider 需要 activate 期创建时用）。 */
  ops:
    | PlatformOps
    | ((ctx: PageMainContext) => PlatformOps | Promise<PlatformOps>);
}

export interface PlatformExtensionModule {
  activate(ctx: PageMainContext): Promise<void>;
  deactivate(): void;
}

/** op 出口的统一错误包装：失败必带 平台.操作 上下文（协议与私有 op 一致）。 */
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

function guardOps(namespace: string, ops: PlatformOps): PlatformOps {
  return Object.fromEntries(
    Object.entries(ops).map(([name, op]) => [
      name,
      {
        ...op,
        invoke: (params: unknown) =>
          opGuard(namespace, name, () => op.invoke(params)),
      },
    ]),
  );
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
      const ops = typeof spec.ops === "function" ? await spec.ops(ctx) : spec.ops;
      gw.register({
        namespace,
        label: ctx.manifest.name,
        tags: spec.tags ?? [],
        ops: guardOps(namespace, ops),
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

// ── feed 协议：内容渠道 + 播放/弹幕 ──────────────────────────────────────────

export interface FeedSpec {
  channels: DataChannel[];
  /** 平台的播放/弹幕能力（无播放能力的平台省略）。 */
  playback?: PlatformPlayback;
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

function findChannel(
  channels: Map<string, DataChannel>,
  key: unknown,
): DataChannel {
  const k = String(key ?? "");
  const channel = channels.get(k);
  if (channel === undefined) {
    throw new Error(`未知渠道 "${k}"`);
  }
  return channel;
}

async function fetchChannel(
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
  const channel = findChannel(channels, p.channel);
  const source = channel.getSource(p.info ?? {});
  if (p.passive === true) {
    if (source.fetchPassive === undefined) {
      throw new Error(`渠道 ${channel.key} 不支持被动采集`);
    }
    const result = await source.fetchPassive();
    if (result === null) throw new Error(`渠道 ${channel.key} 被动采集无数据`);
    return result;
  }
  if (p.cursor !== undefined) {
    if (source.fetchMore === undefined) {
      throw new Error(`渠道 ${channel.key} 不支持翻页`);
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

function playTarget(playback: PlatformPlayback, p: PlayParams): PlayContext {
  let id = typeof p.id === "string" ? p.id.trim() : "";
  let kind = p.kind;
  if (id === "" && typeof p.url === "string" && p.url !== "") {
    const route = matchRoute(playback.routes ?? [], p.url);
    if (route === undefined) throw new Error(`无法从 URL 解析播放路由: ${p.url}`);
    id = route.id;
    kind = route.kind;
  }
  if (id === "") throw new Error(".play 需要 url 或 id");
  if (kind === undefined) {
    kind = playback.resolveVideoPlay !== undefined ? "video" : "live";
  }
  return { id, kind };
}

async function play(playback: PlatformPlayback, params: unknown): Promise<unknown> {
  const p = (params ?? {}) as PlayParams;
  const { id, kind } = playTarget(playback, p);
  if (kind === "live") {
    if (playback.resolveLivePlay === undefined) {
      throw new Error("未实现直播播放解析");
    }
    return { id, streams: await playback.resolveLivePlay(id, p.info) };
  }
  if (playback.resolveVideoPlay === undefined) {
    throw new Error("未实现视频播放解析");
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
  playback: PlatformPlayback,
  p: DanmakuParams,
): { id: string; kind?: "vod" | "live" } {
  let id = typeof p.id === "string" ? p.id.trim() : "";
  let kind = p.kind;
  if (id === "" && typeof p.url === "string" && p.url !== "") {
    const route = matchRoute(playback.routes ?? [], p.url);
    if (route === undefined) throw new Error(`无法从 URL 解析弹幕路由: ${p.url}`);
    id = route.id;
    kind ??= route.kind === "video" ? "vod" : "live";
  }
  if (id === "") throw new Error(".danmaku 需要 url 或 id");
  return { id, ...(kind !== undefined ? { kind } : {}) };
}

function danmaku(playback: PlatformPlayback, params: unknown): unknown {
  const p = (params ?? {}) as DanmakuParams;
  if (playback.getDanmaku === undefined) {
    throw new Error("未实现弹幕能力");
  }
  const { id, kind } = danmakuTarget(playback, p);
  const info: DanmakuInfo = {
    ...(kind !== undefined ? { kind } : {}),
    ...(p.cookie !== undefined ? { cookie: p.cookie } : {}),
  };
  const stream = playback.getDanmaku(id, info);
  return { id, ...(kind !== undefined ? { kind } : {}), subscribe: stream };
}

/**
 * 内容渠道协议（channels / fetch / play / danmaku）。
 *
 * 平台把它作为 ops 的一部分注册；协议之外的私有 op 直接与返回值合并即可。
 */
export function feedOps(spec: FeedSpec): PlatformOps {
  const byKey = new Map(spec.channels.map((c) => [c.key, c] as const));
  const meta = spec.channels.map(channelMeta);

  const ops: PlatformOps = {
    channels: {
      description: "渠道清单（静态描述：key / 名称 / kind / 参数表）",
      invoke: () => meta,
    },
    fetch: {
      description:
        "抓取渠道数据；cursor 翻页（existing=消费方已持有条数，滚动型渠道必需），passive=true 走页面被动采集",
      invoke: (params) => fetchChannel(byKey, params),
    },
  };

  const playback = spec.playback;
  if (playback !== undefined) {
    ops.play = {
      description: "按 url 或 id 解析可播流（视频/直播）",
      invoke: (params) => play(playback, params),
    };
    if (playback.getDanmaku !== undefined) {
      ops.danmaku = {
        description: "按 url 或 id 获取弹幕流（subscribe 返回取消函数）",
        invoke: (params) => danmaku(playback, params),
      };
    }
  }
  return ops;
}

// ── metadata 协议：媒体元数据（douban / bangumi / tmdb 等）─────────────────

/**
 * 媒体元数据协议（search / fetchMetadata / fetchEpisodes），契约见
 * @tabs/platform-contracts 的 MetadataProvider；media-library 按命名空间统一调用。
 */
export function metadataOps(provider: MetadataProvider): PlatformOps {
  return {
    search: {
      description: "按关键词搜索条目",
      invoke: (params) => {
        const p = (params ?? {}) as { keyword?: string; mediaType?: MediaType };
        return provider.search(String(p.keyword ?? ""), p.mediaType ?? "tv");
      },
    },
    fetchMetadata: {
      description: "按平台条目 id 取完整元数据",
      invoke: (params) => {
        const p = (params ?? {}) as { id?: string; mediaType?: MediaType };
        return provider.fetchMetadata(String(p.id ?? ""), p.mediaType ?? "tv");
      },
    },
    fetchEpisodes: {
      description: "取剧集每集元数据（不支持的平台返回空数组）",
      invoke: (params) => {
        const p = (params ?? {}) as { meta?: unknown; id?: string };
        return provider.fetchEpisodes(p.meta as MediaMetadata, String(p.id ?? ""));
      },
    },
  };
}
