/**
 * definePlatformExtension 单测：fake ctx + fake gateway，验证注册机制与两种协议
 * （feedOps / metadataOps），不发网络、不碰真实 Electron。
 */

import { describe, expect, test } from "bun:test";

import type { MediaMetadata, MetadataProvider, MediaType } from "@tabs/platform-contracts";

import type { DataChannel, Item } from "../src/data.ts";
import type { PlatformPlayback } from "../src/runtime/playback.ts";
import {
  definePlatformExtension,
  feedOps,
  metadataOps,
} from "../src/runtime/extension.ts";

const item: Item = {
  id: "demo:1",
  sourceId: "demo",
  kind: "article",
  title: "hello",
  fetchedAt: 1,
};

/** 记录 fetchMore / getDanmaku 收到的上下文，供 existing / kind 透传断言。 */
const received: { fetchMoreCtx?: { existing?: number }; danmakuInfo?: Record<string, unknown> } = {};

const channel: DataChannel = {
  key: "demo:items",
  name: "Demo 条目",
  kind: "article",
  sourceInfoTpl: [{ key: "q", label: "关键词", required: true }],
  getSource: (info) => ({
    fetch: async () => ({
      items: [{ ...item, title: `${item.title} ${info.q ?? ""}`.trim() }],
      total: 1,
    }),
    fetchMore: async (cursor, ctx) => {
      received.fetchMoreCtx = ctx;
      return {
        items: [{ ...item, id: `demo:next:${cursor ?? "0"}` }],
        cursor: "2",
        hasMore: true,
      };
    },
    fetchPassive: async () => null,
  }),
};

const playback: PlatformPlayback = {
  platform: "bilibili",
  routes: [
    {
      test: /(?:^|[./])bilibili\.com\/video\/(BV\w+)/,
      kind: "video",
      extractId: (u) => u.match(/\/video\/(BV\w+)/)?.[1] ?? null,
    },
    {
      test: /(?:^|[./])live\.bilibili\.com\/(\d+)/,
      kind: "live",
      extractId: (u) => u.match(/live\.bilibili\.com\/(\d+)/)?.[1] ?? null,
    },
  ],
  resolveVideoPlay: async (id) => ({
    streams: [{ url: `https://cdn/${id}.m3u8`, format: "hls" }],
    chapters: [{ t: 0, label: "开场" }],
  }),
  resolveLivePlay: async (id) => [{ url: `https://live/${id}.flv` }],
  getDanmaku: (id, opts) => {
    received.danmakuInfo = { id, ...(opts ?? {}) };
    return (onItems) => {
      onItems([{ text: "hi" }]);
      return () => undefined;
    };
  },
};

interface Registration {
  namespace: string;
  label: string;
  tags?: string[];
  ops: Record<string, { invoke: (params: unknown) => unknown }>;
}

function makeCtx(): {
  ctx: Parameters<typeof definePlatformExtension>[0] extends never ? never : any;
  getReg: () => Registration;
} {
  let reg: Registration | undefined;
  const gateway = {
    register(r: Registration): void {
      reg = r;
    },
    call(): void {},
    list(): unknown[] {
      return [];
    },
  };
  const ctx = {
    manifest: {
      id: "platform-demo",
      name: "Demo 平台",
      version: "0.1.0",
      kind: "service",
      main: "main.js",
      inject: ["api.gateway"],
    },
    pageRoot: process.cwd(),
    dataDir: process.cwd(),
    appPath: process.cwd(),
    ipc: { handle() {}, on() {}, removeHandler() {}, removeListener() {} },
    getWindow: () => undefined,
    log: () => undefined,
    provide: () => undefined,
    get: () => undefined,
    require: () => gateway,
  };
  return {
    ctx,
    getReg: () => {
      if (reg === undefined) throw new Error("gateway.register 未被调用");
      return reg;
    },
  };
}

describe("definePlatformExtension", () => {
  test("注册 channels/fetch/play/danmaku 四个 op", async () => {
    const { ctx, getReg } = makeCtx();
    const mod = definePlatformExtension({
      tags: ["platform.data"],
      ops: feedOps({ channels: [channel], playback }),
    });
    await mod.activate(ctx);
    const reg = getReg();
    expect(reg.namespace).toBe("demo");
    expect(reg.label).toBe("Demo 平台");
    expect(reg.tags).toEqual(["platform.data"]);
    expect(Object.keys(reg.ops).sort()).toEqual([
      "channels",
      "danmaku",
      "fetch",
      "play",
    ]);
  });

  test("channels 只暴露静态描述", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({ ops: feedOps({ channels: [channel] }) }).activate(ctx);
    const channels = (await getReg().ops.channels!.invoke({})) as Array<
      Record<string, unknown>
    >;
    expect(channels).toHaveLength(1);
    expect(channels[0]).toMatchObject({
      key: "demo:items",
      name: "Demo 条目",
      kind: "article",
    });
    expect("getSource" in channels[0]!).toBe(false);
  });

  test("fetch 分发 fetch / fetchMore / passive", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({ ops: feedOps({ channels: [channel] }) }).activate(ctx);
    const fetch = getReg().ops.fetch!.invoke;

    const first = (await fetch({ channel: "demo:items", info: { q: "x" } })) as {
      items: Item[];
      total?: number;
    };
    expect(first.items[0]?.title).toBe("hello x");
    expect(first.total).toBe(1);

    const next = (await fetch({ channel: "demo:items", cursor: "1" })) as {
      items: Item[];
      cursor?: string;
      hasMore?: boolean;
    };
    expect(next.items[0]?.id).toBe("demo:next:1");
    expect(next.cursor).toBe("2");
    expect(next.hasMore).toBe(true);

    // existing 透传：滚动型渠道首次翻页的起点必须由消费方给出。
    await fetch({ channel: "demo:items", cursor: "1", existing: 42 });
    expect(received.fetchMoreCtx).toEqual({ existing: 42 });
    await fetch({ channel: "demo:items", cursor: "1", existing: -1 });
    expect(received.fetchMoreCtx).toBeUndefined();

    await expect(fetch({ channel: "demo:items", passive: true })).rejects.toThrow(
      "被动采集无数据",
    );
    await expect(fetch({ channel: "missing" })).rejects.toThrow("未知渠道");
  });

  test("play 按 id/kind 分发视频与直播", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({
      ops: feedOps({ channels: [channel], playback }),
    }).activate(ctx);
    const play = getReg().ops.play!.invoke;

    const video = (await play({ id: "BV1", kind: "video" })) as {
      id: string;
      streams: Array<{ url: string }>;
      chapters?: Array<{ label: string }>;
    };
    expect(video.id).toBe("BV1");
    expect(video.streams[0]?.url).toBe("https://cdn/BV1.m3u8");
    expect(video.chapters?.[0]?.label).toBe("开场");

    const live = (await play({ id: "123", kind: "live" })) as {
      streams: Array<{ url: string }>;
    };
    expect(live.streams[0]?.url).toBe("https://live/123.flv");
  });

  test("play/danmaku 按 url 走平台自带路由", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({
      ops: feedOps({ channels: [channel], playback }),
    }).activate(ctx);

    const video = (await getReg().ops.play!.invoke({
      url: "https://www.bilibili.com/video/BV1xx411c7mD",
    })) as { id: string; streams: Array<{ url: string }> };
    expect(video.id).toBe("BV1xx411c7mD");
    expect(video.streams[0]?.url).toBe("https://cdn/BV1xx411c7mD.m3u8");

    const live = (await getReg().ops.play!.invoke({
      url: "https://live.bilibili.com/6",
    })) as { id: string; streams: Array<{ url: string }> };
    expect(live.id).toBe("6");
    expect(live.streams[0]?.url).toBe("https://live/6.flv");

    const dm = (await getReg().ops.danmaku!.invoke({
      url: "https://www.bilibili.com/video/BV1xx411c7mD",
    })) as { kind: string };
    expect(dm.kind).toBe("vod");

    await expect(
      getReg().ops.play!.invoke({ url: "https://example.com/nope" }),
    ).rejects.toThrow("无法从 URL 解析播放路由");
  });

  test("danmaku 返回可订阅流；id-only 不硬填 kind（交给平台缺省）", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({
      ops: feedOps({ channels: [channel], playback }),
    }).activate(ctx);
    const result = (await getReg().ops.danmaku!.invoke({
      id: "123",
      kind: "live",
    })) as {
      id: string;
      kind: string;
      subscribe: (cb: (items: Array<{ text: string }>) => void) => () => void;
    };
    expect(result.id).toBe("123");
    expect(result.kind).toBe("live");
    const batches: Array<Array<{ text: string }>> = [];
    const off = result.subscribe((items) => batches.push(items));
    expect(batches[0]?.[0]?.text).toBe("hi");
    off();

    // id-only：kind 省略，平台侧（如 bili 的 ?? "vod"）拿到 undefined 才能生效。
    const idOnly = (await getReg().ops.danmaku!.invoke({ id: "BV1" })) as {
      kind?: string;
    };
    expect(idOnly.kind).toBeUndefined();
    expect(received.danmakuInfo).toEqual({ id: "BV1" });
  });

  test("私有 op 可与协议共存，失败统一带 平台.操作 上下文", async () => {
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({
      tags: ["demo.private"],
      ops: {
        ...feedOps({ channels: [channel] }),
        whoami: { description: "demo", invoke: () => ({ ok: true }) },
        explode: {
          invoke: () => {
            throw new Error("boom");
          },
        },
      },
    }).activate(ctx);
    expect(getReg().tags).toEqual(["demo.private"]);
    expect(await getReg().ops.whoami!.invoke({})).toEqual({ ok: true });
    await expect(getReg().ops.explode!.invoke({})).rejects.toThrow(
      "[demo.explode] boom",
    );
  });

  test("metadataOps 按 provider 契约暴露 search/fetchMetadata/fetchEpisodes", async () => {
    const calls: string[] = [];
    const provider: MetadataProvider = {
      source: "demo",
      search: async (keyword: string, mediaType: MediaType) => {
        calls.push(`search:${keyword}:${mediaType}`);
        return [];
      },
      fetchMetadata: async (id: string, mediaType: MediaType) => {
        calls.push(`meta:${id}:${mediaType}`);
        return { title: id } as unknown as MediaMetadata;
      },
      fetchEpisodes: async (_meta: MediaMetadata, id: string) => {
        calls.push(`eps:${id}`);
        return [];
      },
    };
    const { ctx, getReg } = makeCtx();
    await definePlatformExtension({
      tags: ["media.metadata"],
      ops: () => metadataOps(provider),
    }).activate(ctx);

    expect(Object.keys(getReg().ops).sort()).toEqual([
      "fetchEpisodes",
      "fetchMetadata",
      "search",
    ]);
    await getReg().ops.search!.invoke({ keyword: "k" });
    await getReg().ops.fetchMetadata!.invoke({ id: "42" });
    await getReg().ops.fetchEpisodes!.invoke({ meta: {}, id: "42" });
    expect(calls).toEqual(["search:k:tv", "meta:42:tv", "eps:42"]);
  });
});
