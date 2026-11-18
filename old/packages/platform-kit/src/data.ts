/**
 * 平台数据渠道装配工厂（契约类型在 @tabs/platform-contracts）。
 *
 * 与旧 RSS 渠道的差别只有输出：渠道的 fetch 直接返回结构化 `Item[]`，
 * fetchMore 也返回结构化分页结果；不经过 RSS/XML 序列化。
 *
 * 契约（DataChannel / DataSource / FetchResult …）从 contracts 统一 re-export，
 * 平台包仍可只从本模块取类型 + 工厂。
 */

import type {
  DataChannel,
  DataSource,
  FetchPage,
  FetchResult,
  Item,
  Kind,
  Loginable,
  Pageable,
  SourceInfo,
  SourceInfoField,
} from "@tabs/platform-contracts";

export * from "@tabs/platform-contracts";

/**
 * XML 时代的频道标题/链接选项。
 * @deprecated 渠道输出已是结构化数据；保留类型只为兼容旧渠道声明，值是死配置。
 */
export interface SerializeOptions {
  channelTitle?: string;
  channelLink?: string;
  total?: number;
}

/** api channel 的 fetch 装配：直接返回结构化结果。 */
export function apiFetch(
  fetchResult: () => Promise<FetchResult>,
  _channelOptions?: () => SerializeOptions,
): () => Promise<FetchResult> {
  return async () => {
    const { items, total } = await fetchResult();
    return total !== undefined ? { items, total } : { items };
  };
}

/** 无 total 的渠道适配到统一 FetchResult。 */
export function apiFetchItems(
  fetchItems: () => Promise<Item[]>,
  _channelOptions?: () => SerializeOptions,
): () => Promise<FetchResult> {
  return async () => ({ items: await fetchItems() });
}

/**
 * api channel 的 fetchMore 装配：数值游标分页（起步 first、步进 step）。
 * 本页 0 条 ⇒ 没有更多（过滤过条目的渠道应返回显式 hasMore）。
 */
export function apiFetchMore(
  fetchPage: (
    n: number,
  ) => Promise<Item[] | { items: Item[]; hasMore?: boolean; total?: number }>,
  _channelOptions: (() => SerializeOptions) | undefined,
  opts: { first: number; step: number },
): (cursor?: string) => Promise<FetchPage> {
  return async (cursor) => {
    const n = cursor ? Number(cursor) : opts.first;
    const r = await fetchPage(n);
    const items = Array.isArray(r) ? r : r.items;
    const hasMore = Array.isArray(r) ? undefined : r.hasMore;
    const total = Array.isArray(r) ? undefined : r.total;
    return {
      items,
      ...(total !== undefined ? { total } : {}),
      ...((hasMore ?? items.length > 0) ? { cursor: String(n + opts.step) } : {}),
      ...(hasMore !== undefined ? { hasMore } : {}),
    };
  };
}

/**
 * 「单端点 GET → 列表 → 映射」同形渠道的声明式规格。
 * 渠道差异只剩 key/name/kind/tpl + fetchPage 纯映射。
 */
export interface ApiChannelSpec {
  key: string;
  name: string;
  kind: Kind;
  /** 实例化参数表单（供 UI）；无参渠道不声明。 */
  sourceInfoTpl?: SourceInfoField[];
  /** 无需用户输入即可订阅的默认参数；无参渠道不声明。 */
  defaultInfo?: SourceInfo;
  /** 平台级扫码登录（popular 等 Loginable 渠道）；isLoginable 据此探测。 */
  scanLogin?: Loginable["scanLogin"];
  /** 启动快照（浏览器登录的渠道）；按方法存在性探测。 */
  readLoginCookie?: DataChannel["readLoginCookie"];
  /** @deprecated 死配置（XML 时代字段），仅为兼容旧调用点保留。 */
  channelTitle?: (info: SourceInfo) => string;
  /** @deprecated 死配置（XML 时代字段），仅为兼容旧调用点保留。 */
  channelLink?: string | ((info: SourceInfo) => string);
  /** 取第 page 页条目（页码从 1 起）。 */
  fetchPage: (
    info: SourceInfo,
    page: number,
  ) => Promise<Item[] | { items: Item[]; hasMore?: boolean; total?: number }>;
  /** 数值翻页（省略 = 单页无翻页）。first：首页之后的起始页；step：页步进。 */
  pageable?: { first: number; step: number };
}

/** 声明式 api channel 装配：getSource 接线与分页样板收敛到这一处。 */
export function makeApiChannel(spec: ApiChannelSpec): DataChannel {
  const source = (info: SourceInfo): DataSource & Partial<Pageable> => {
    const fetchResult = async (): Promise<FetchResult> => {
      const r = await spec.fetchPage(info, 1);
      return Array.isArray(r) ? { items: r } : { items: r.items, ...(r.total !== undefined ? { total: r.total } : {}) };
    };
    const s: DataSource & Partial<Pageable> = { fetch: fetchResult };
    if (spec.pageable) {
      s.fetchMore = apiFetchMore(
        (page) => spec.fetchPage(info, page),
        undefined,
        spec.pageable,
      );
    }
    return s;
  };
  return {
    key: spec.key,
    name: spec.name,
    kind: spec.kind,
    ...(spec.sourceInfoTpl ? { sourceInfoTpl: spec.sourceInfoTpl } : {}),
    ...(spec.defaultInfo ? { defaultInfo: spec.defaultInfo } : {}),
    getSource: source,
    ...(spec.scanLogin ? { scanLogin: spec.scanLogin } : {}),
    ...(spec.readLoginCookie ? { readLoginCookie: spec.readLoginCookie } : {}),
  } as DataChannel;
}
