/**
 * 媒体 IPC 绑定：channel → MediaService 方法（独立应用的 ipcMain 与宿主页面的
 * ctx.ipc 共用同一张表，保证两种形态的 renderer 契约完全一致）。
 * sender 鉴权：allowedOrigin 设置后只接受该 URL 前缀的调用（页面模式下防其他
 * 已安装页面调用写接口）。返回 unbind；宿主 ctx.ipc 的 channel 由宿主 tracker
 * 兜底清理，unbind 只是让 deactivate 更早释放。
 */

import type { PageIpc, PageIpcEvent, PageIpcHandler } from "@host/sdk";
import type { MediaService } from "./service";

type HandleBinding = {
  type: "handle";
  channel: string;
  call: (service: MediaService, ...args: unknown[]) => unknown;
};

type OnBinding = {
  type: "on";
  channel: string;
  call: (service: MediaService, ...args: unknown[]) => void;
};

/** 渲染侧 channel 契约（renderer/preload 不随拆分变化） */
const BINDINGS: readonly (HandleBinding | OnBinding)[] = [
  {
    type: "handle",
    channel: "library:pick",
    call: (s) => s.pickLibrary(),
  },
  {
    type: "handle",
    channel: "library:scan",
    call: (s, root) => s.scanLibrary(root as string),
  },
  {
    type: "handle",
    channel: "show:detail",
    call: (s, dir) => s.showDetail(dir as string),
  },
  {
    type: "handle",
    channel: "image:data-url",
    call: (s, path) => s.imageToDataUrl(path as string),
  },
  {
    type: "on",
    channel: "shell:reveal",
    call: (s, path) => s.reveal(path as string),
  },
  {
    type: "handle",
    channel: "search:metadata",
    call: (s, source, keyword, mediaType) =>
      s.searchMetadata(source as never, keyword as string, mediaType as never),
  },
  {
    type: "handle",
    channel: "scrape:preview",
    call: (s, source, id, mediaType) =>
      s.scrapePreview(source as never, id as string, mediaType as never),
  },
  {
    type: "handle",
    channel: "scrape:apply",
    call: (s, dir, source, id, mediaType) =>
      s.scrapeApply(
        dir as string,
        source as never,
        id as string,
        mediaType as never,
      ),
  },
  {
    type: "handle",
    channel: "nfo:read",
    call: (s, dir) => s.readNfo(dir as string),
  },
  {
    type: "handle",
    channel: "nfo:write",
    call: (s, dir, metadata) => s.writeNfo(dir as string, metadata as never),
  },
  {
    type: "handle",
    channel: "organize:plan",
    call: (s, root, dir) => s.organizePlan(root as string, dir as string),
  },
  {
    type: "handle",
    channel: "organize:apply",
    call: (s, root, dir) => s.organizeApply(root as string, dir as string),
  },
  {
    type: "handle",
    channel: "cache:clear",
    call: (s) => s.cacheClear(),
  },
];

export function bindMediaIpc(
  ipc: PageIpc,
  service: MediaService,
  allowedOrigin?: string,
): () => void {
  const allowed = (event: PageIpcEvent): boolean =>
    allowedOrigin === undefined ||
    event.senderFrame?.url.startsWith(allowedOrigin) === true;

  const handled: string[] = [];
  const listened: { channel: string; listener: PageIpcHandler }[] = [];

  for (const binding of BINDINGS) {
    if (binding.type === "handle") {
      const listener: PageIpcHandler = (event, ...args) => {
        if (!allowed(event))
          throw new Error(
            `IPC not allowed for ${event.senderFrame?.url ?? "unknown sender"}`,
          );
        return binding.call(service, ...args);
      };
      ipc.handle(binding.channel, listener);
      handled.push(binding.channel);
    } else {
      const listener: PageIpcHandler = (event, ...args) => {
        if (!allowed(event)) return;
        binding.call(service, ...args);
      };
      ipc.on(binding.channel, listener);
      listened.push({ channel: binding.channel, listener });
    }
  }

  return () => {
    for (const channel of handled) ipc.removeHandler(channel);
    for (const { channel, listener } of listened) {
      ipc.removeListener(channel, listener);
    }
  };
}
