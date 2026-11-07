/**
 * api-gateway —— 统一平台 API 注册中心（api.gateway）。
 *
 * 所有 platform-* 插件把能力以「平台.操作」注册到这里（如 `tmdb.search`、
 * `bili.channels`）；消费方（域服务与未来的页面组件）经 call/list 使用。
 * 本组件只有注册表与分发，**不含** rss 域的订阅/抓取/存储/事件 —— 那些属于
 * rss-service 数据服务进程。
 *
 * 命名空间 = 平台 id（单段小写，平台就是平台，不带域前缀）；域归类用可选
 * tags（如 ["media.metadata"]），media-library 据此过滤元数据渠道。
 *
 * 为什么不是 GraphQL（决策记录）：
 * - 查询形态是过程式（搜索/按 id 取详情/分页），无图遍历与字段裁剪需求；
 * - 消费端是宿主 IPC 而非 HTTP，GraphQL 生态（持久化查询/缓存/devtools）
 *   用不上，只多一层解析；
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
  /** 平台 id（单段小写，如 "tmdb" / "bili"）；操作全名 = <namespace>.<op> */
  readonly namespace: string;
  /** 人读标签（如 "TMDB"） */
  readonly label: string;
  /** 域归类（可选，如 ["media.metadata"]）；仅内省/过滤用，不影响命名 */
  readonly tags?: string[];
  /** 操作表：名字 → handler（同 namespace 重复注册整体覆盖） */
  readonly ops: Record<string, ApiOpHandler>;
}

export interface ApiInfo {
  namespace: string;
  label: string;
  tags?: string[];
  ops: Array<{ name: string; description?: string }>;
}

const NAMESPACE_RE = /^[a-z][a-z0-9-]*$/;
const OP_RE = /^[a-z][a-zA-Z0-9-]*$/;
/** 网关自留（自检）与保留字，平台不得占用 */
const RESERVED = new Set(["gateway"]);

export function activate(ctx: PageMainContext): void {
  const apis = new Map<string, ApiRegistration>();

  const register = (reg: ApiRegistration): void => {
    if (typeof reg?.namespace !== "string" || !NAMESPACE_RE.test(reg.namespace)) {
      throw new Error(`api.gateway: 非法命名空间 "${String(reg?.namespace)}"（平台 id，单段小写，如 tmdb / bili）`);
    }
    if (RESERVED.has(reg.namespace)) {
      throw new Error(`api.gateway: 命名空间 "${reg.namespace}" 为网关保留字`);
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
    if (dot <= 0 || dot === op.length - 1) {
      throw new Error(`api.gateway: 非法操作名 "${op}"（应为 平台.操作，如 tmdb.search）`);
    }
    const namespace = op.slice(0, dot);
    const name = op.slice(dot + 1);
    const handler = apis.get(namespace)?.ops[name];
    if (handler === undefined) {
      const known = [...apis.keys()].join(", ");
      throw new Error(`api.gateway: 未注册操作 "${op}"（已注册平台：${known || "无"}）`);
    }
    return handler.invoke(params);
  };

  const list = (): ApiInfo[] =>
    [...apis.values()]
      .map((reg) => ({
        namespace: reg.namespace,
        label: reg.label,
        ...(Array.isArray(reg.tags) ? { tags: reg.tags } : {}),
        ops: Object.entries(reg.ops).map(([name, h]) => ({
          name,
          ...(h.description !== undefined ? { description: h.description } : {}),
        })),
      }))
      .sort((a, b) => a.namespace.localeCompare(b.namespace));

  ctx.provide("api.gateway", { register, call, list }, { expose: ["register", "call", "list"] });

  // 自环自检：注册即调用一个内置操作，smoke 日志可见注册/分发链路健康
  const self = {
    namespace: "gateway",
    label: "网关自检",
    ops: {
      ping: {
        description: "回显参数，验证注册与分发链路",
        invoke: (params: unknown) => ({ pong: true, params }),
      },
    },
  };
  apis.set(self.namespace, self as ApiRegistration);
  void call("gateway.ping", { check: 1 }).then(
    (r) => ctx.log(`self-check ok: ${JSON.stringify(r)}`),
    (err) => ctx.log(`self-check FAILED: ${String(err)}`),
  );
}

export function deactivate(): void {
  // 注册表随组件实例丢弃；依赖组件 reload 后重新注册
}
