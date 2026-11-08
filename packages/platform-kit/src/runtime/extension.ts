/**
 * 平台组件入口工厂（数据接口版）：把该平台的渠道与播放/弹幕能力直接注册到
 * api.gateway，不再交给 rss 数据服务进程。
 *
 * 注册的操作（namespace = 组件 id 去掉 platform- 前缀）：
 *   - `<ns>.channels` —— 渠道静态描述（key/name/kind/参数表）
 *   - `<ns>.fetch`    —— 抓取：{ channel, info?, cursor?, passive? } → FetchResult/FetchPage
 *   - `<ns>.play`     —— 播放：{ url | id, kind?, info? } → { id, streams, chapters? }
 *   - `<ns>.danmaku`  —— 弹幕：{ url | id, kind?, cookie? } → { id, kind, subscribe }
 *
 * 渠道执行在宿主主进程内（appHost 由 runtime/electron-host 安装）；浏览器型渠道
 * 需要组件在 inject 里声明 rss.browser，由 CDP 代理构造 BrowserBackend。
 */

import type { PageMainContext } from "@host/sdk";

import type { DataChannel, FetchResult, SourceInfo } from "../data.ts";
import { installPlatformAppHost } from "./electron-host.ts";
import {
  matchRoute,
  type DanmakuInfo,
  type PlatformPlayback,
} from "./playback.ts";

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
        const svc = ctx.get<BrowserService>("rss.browser");
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
          invoke: (params) => fetchChannel(namespace, byKey, params),
        },
      };

      if (spec.playback !== undefined) {
        const playback = spec.playback;
        ops.play = {
          description: "按 url 或 id 解析可播流（视频/直播）",
          invoke: (params) => play(namespace, playback, params),
        };
        if (playback.getDanmaku !== undefined) {
          ops.danmaku = {
            description: "按 url 或 id 获取弹幕流（subscribe 返回取消函数）",
            invoke: (params) => danmaku(namespace, playback, params),
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
