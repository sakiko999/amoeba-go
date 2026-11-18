/**
 * renderer 侧 IPC 封装：window.media（preload 注入）+ 海报 data-url 模块级缓存。
 */

import { useEffect, useState } from "react";
import type { MediaApi } from "@tabs/media-library";

export function media(): MediaApi {
  const api = window.media;
  if (!api) throw new Error("window.media is missing — preload did not run");
  return api;
}

const posterCache = new Map<string, string | null>();

/** undefined = 无海报路径；null = 加载完成但失败（显示占位） */
export function usePoster(path: string | undefined): string | null | undefined {
  const [url, setUrl] = useState<string | null | undefined>(
    path === undefined ? undefined : posterCache.get(path),
  );
  useEffect(() => {
    if (path === undefined) return;
    const cached = posterCache.get(path);
    if (cached !== undefined) {
      setUrl(cached);
      return;
    }
    let alive = true;
    void media()
      .imageDataUrl(path)
      .then((data) => {
        posterCache.set(path, data);
        if (alive) setUrl(data);
      });
    return () => {
      alive = false;
    };
  }, [path]);
  return url;
}
