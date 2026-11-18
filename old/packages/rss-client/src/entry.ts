/**
 * RSS 数据客户端组件 main（CJS）：inject rss.endpoint，provide rss.client。
 *
 * 消费方（未来的 RSS 页面组件）不需要自己处理服务协议细节：
 *   - call(method, params)   —— POST /v1/rpc，Auth 头由组件注入（owner token
 *                               不过组件边界，endpoint() 也刻意不返回 token）
 *   - endpoint()             —— { baseUrl, port }（给需要直连 WS 事件流的 renderer 用）
 *   - health()               —— /v1/health
 *
 * 服务（rss-service）reload 时宿主会级联停用本组件并在服务恢复后重新激活，
 * 因此 call 永远按**当前** endpoint 重新解析（不缓存 baseUrl 之外的连接）。
 */

import type { PageMainContext } from "@host/sdk";

export const SERVICE_NAME = "rss.client";

const RPC_TIMEOUT_MS = 30_000;

/** rss-service 组件提供的端点形状（只取用得到的字段，避免跨包类型依赖） */
interface EndpointProvider {
  endpoint(): { baseUrl: string; port: number; token: string };
}

interface RpcEnvelope<T> {
  ok: boolean;
  result?: T;
  rev?: number;
  error?: { kind?: string; message?: string };
}

export interface RssClient {
  endpoint(): { baseUrl: string; port: number };
  call<T = unknown>(method: string, params?: Record<string, unknown>): Promise<T>;
  health(): Promise<{ ok: boolean; rev: number; uptimeMs: number; jobs: number }>;
}

let client: RssClient | undefined;

export function activate(ctx: PageMainContext): void {
  const ep = ctx.require<EndpointProvider>("rss.endpoint");
  const info = ep.endpoint();
  const authed = (path: string, init?: RequestInit): Promise<Response> =>
    fetch(`${info.baseUrl}${path}`, {
      ...init,
      headers: { authorization: `Bearer ${info.token}`, ...init?.headers },
    });

  client = {
    endpoint: () => ({ baseUrl: info.baseUrl, port: info.port }),

    async call<T>(method: string, params?: Record<string, unknown>): Promise<T> {
      const res = await authed("/v1/rpc", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ method, params: params ?? {} }),
        signal: AbortSignal.timeout(RPC_TIMEOUT_MS),
      });
      const body = (await res.json()) as RpcEnvelope<T>;
      if (!body.ok) {
        throw new Error(
          `[${body.error?.kind ?? `http-${res.status}`}] ${body.error?.message ?? "rpc failed"}`,
        );
      }
      return body.result as T;
    },

    async health() {
      const res = await authed("/v1/health");
      if (!res.ok) throw new Error(`/v1/health ${res.status}`);
      return (await res.json()) as {
        ok: boolean;
        rev: number;
        uptimeMs: number;
        jobs: number;
      };
    },
  };

  ctx.provide(SERVICE_NAME, client, {
    expose: ["endpoint", "call", "health"],
  });
  ctx.log(`rss.client ready: ${info.baseUrl}`);
}

export function deactivate(): void {
  client = undefined;
}

/** 供其他组件/测试使用：当前实例（仅在 active 期间非空） */
export function currentClient(): RssClient | undefined {
  return client;
}
