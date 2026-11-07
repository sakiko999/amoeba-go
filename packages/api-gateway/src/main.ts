/**
 * api-gateway —— 统一平台 API 注册中心（api.gateway）。
 *
 * 所有 platform-* 插件（rss 平台 / media 平台）把自己的能力以
 * 「命名空间.操作」注册到这里；消费方（域服务与未来的页面组件）经
 * call/list 使用。本组件只有注册表与分发，**不含** rss 域的订阅/抓取/
 * 存储/事件 —— 那些属于 rss-service 数据服务进程。
 *
 * 为什么不是 GraphQL（决策记录）：
 * - 查询形态是过程式（搜索/按 id 取详情/分页），无图遍历与字段裁剪需求；
 * - 消费端是宿主 IPC 而非 HTTP，GraphQL 生态（持久化查询/缓存/devtools）
 *   用不上，只多一层解析；
 * - 组件产物保持零运行时依赖（graphql-js + schema 拼接会打破它）；
 * - 演进路径：注册表 handler 形状 (params) => Promise<result> 与 GraphQL
 *   resolver 同构，未来需要时可在网关上加「注册表 → SDL」层，插件不改。
 */

import type { PageMainContext } from "@host/sdk";

export interface ApiOpHandler {
  /** 人读说明（list() 内省用） */
  description?: string;
  invoke(params: unknown): Promise<unknown> | unknown;
}

export interface ApiRegistration {
  /** 命名空间，形如 "media.tmdb" / "rss.bili"（域.平台，全小写） */
  readonly namespace: string;
  /** 人读标签（如 "媒体元数据 · TMDB"） */
  readonly label: string;
  /** 操作表：名字 → handler（同 namespace 重复注册整体覆盖） */
  readonly ops: Record<string, ApiOpHandler>;
}

export interface ApiInfo {
  namespace: string;
  label: string;
  ops: Array<{ name: string; description?: string }>;
}

const NAMESPACE_RE = /^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*$/;
const OP_RE = /^[a-z][a-zA-Z0-9-]*$/;

export function activate(ctx: PageMainContext): void {
  const apis = new Map<string, ApiRegistration>();

  const register = (reg: ApiRegistration): void => {
    if (typeof reg?.namespace !== "string" || !NAMESPACE_RE.test(reg.namespace)) {
      throw new Error(`api.gateway: 非法命名空间 "${String(reg?.namespace)}"（应为 域.平台，如 media.tmdb）`);
    }
    if (reg.ops === undefined || typeof reg.ops !== "object") {
      throw new Error(`api.gateway: ${reg.namespace} 未提供 ops`);
    }
    for (const name of Object.keys(reg.ops)) {
      if (!OP_RE.test(name)) {
        throw new Error(`api.gateway: ${reg.namespace} 非法操作名 "${name}"`);
      }
      if (typeof reg.ops[name]?.invoke !== "function") {
        throw new Error(`api.gateway: ${reg.namespace}.${name} 缺少 invoke`);
      }
    }
    apis.set(reg.namespace, reg);
    ctx.log(`api registered: ${reg.namespace} (${Object.keys(reg.ops).join(", ")})`);
  };

  const call = async (op: string, params: unknown = {}): Promise<unknown> => {
    const dot = op.indexOf(".");
    const slash = op.indexOf(".", dot + 1);
    if (dot <= 0 || slash <= dot + 1) {
      throw new Error(`api.gateway: 非法操作名 "${op}"（应为 域.平台.操作，如 media.tmdb.search）`);
    }
    const namespace = op.slice(0, slash);
    const name = op.slice(slash + 1);
    const reg = apis.get(namespace);
    const handler = reg?.ops[name];
    if (handler === undefined) {
      const known = [...apis.keys()].join(", ");
      throw new Error(`api.gateway: 未注册操作 "${op}"（已注册：${known || "无"}）`);
    }
    return handler.invoke(params);
  };

  const list = (): ApiInfo[] =>
    [...apis.values()]
      .map((reg) => ({
        namespace: reg.namespace,
        label: reg.label,
        ops: Object.entries(reg.ops).map(([name, h]) => ({
          name,
          ...(h.description !== undefined ? { description: h.description } : {}),
        })),
      }))
      .sort((a, b) => a.namespace.localeCompare(b.namespace));

  ctx.provide("api.gateway", { register, call, list }, { expose: ["register", "call", "list"] });

  // 自环自检：注册即调用一个内置操作，smoke 日志可见注册/分发链路健康
  register({
    namespace: "api.self",
    label: "网关自检",
    ops: {
      ping: {
        description: "回显参数，验证注册与分发链路",
        invoke: (params) => ({ pong: true, params }),
      },
    },
  });
  void call("api.self.ping", { check: 1 }).then(
    (r) => ctx.log(`self-check ok: ${JSON.stringify(r)}`),
    (err) => ctx.log(`self-check FAILED: ${String(err)}`),
  );
}

export function deactivate(): void {
  // 注册表随组件实例丢弃；依赖组件 reload 后重新注册
}
