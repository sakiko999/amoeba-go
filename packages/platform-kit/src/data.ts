/**
 * 平台数据渠道契约 + 装配工厂（项目自有，不再依赖 amoeba 的 crawler/xml）。
 *
 * 与旧 RSS 渠道的差别只有输出：渠道的 fetch 直接返回结构化 `Item[]`，
 * fetchMore 也返回结构化分页结果；不经过 RSS/XML 序列化。
 *
 * 统一数据模型（Item/Stream/Author/...）定义在 @tabs/platform-contracts，
 * 这里 re-export 以保持平台包的单一导入面。
 */

import type { Item, Kind } from "@tabs/platform-contracts";

export * from "@tabs/platform-contracts";

/** 渠道实例化参数（如 `{ uid }` / `{ roomIds }`）。 */
export type SourceInfo = Record<string, string>;

/** 渠道参数字段（供 UI 生成"新增订阅"表单）。 */
export interface SourceInfoField {
  key: string;
  label: string;
  required?: boolean;
  placeholder?: string;
}

/** 渠道一次抓取的返回：条目 + 上游真实总数（可选）。 */
export interface FetchResult {
  items: Item[];
  total?: number;
}

/** 翻页结果：条目 + 下一页游标 + 是否还有更多（+ 可选总数）。 */
export interface FetchPage extends FetchResult {
  cursor?: string;
  hasMore?: boolean;
}

/** 实例化后的数据源（行为载体）。 */
export interface DataSource {
  fetch(): Promise<FetchResult>;
  /**
   * 翻页。cursor 由上次返回给出；首次翻页不传（渠道自行处理两步法）。
   * 返回的 cursor 省略 = 没有更多；hasMore 显式表达「还有下一页」。
   */
  fetchMore?(cursor?: string, ctx?: { existing: number }): Promise<FetchPage>;
  /** 被动采集（LX）：读页面 SSR / 网络缓冲；返回 null = 不可用（绝不回退 fetch）。 */
  fetchPassive?(): Promise<FetchResult | null>;
}

/**
 * 一个数据渠道 —— 纯描述。实例化出的 source 才承担抓取能力。
 */
export interface DataChannel {
  /** 渠道唯一 key（如 "bili:square"、"live:huya"）。 */
  readonly key: string;
  /** 人类可读名称。 */
  readonly name: string;
  /** 该 channel 产出的 item 默认 kind。 */
  readonly kind: Kind;
  /** 实例化 source 需要的参数字段（供 UI 生成表单）。无参 channel 不声明。 */
  readonly sourceInfoTpl?: SourceInfoField[];
  /** 带默认参数的实例（可选）。存在 = 无需用户输入即可订阅一个默认实例。 */
  readonly defaultInfo?: SourceInfo;
  /** 该渠道的正常路径是浏览器被动采集（导航 + 读页面 SSR / CDP 缓冲）。 */
  readonly passive?: boolean;
  /** 启动快照能力（可选）：从浏览器 profile 只读当前登录 cookie。 */
  readLoginCookie?(browser: unknown): Promise<LoginCookieSnapshot | null>;
  /** 按 info 实例化一个可抓取的数据源。 */
  getSource(info: SourceInfo): DataSource;
}

/** 扫码登录结果。cookie 为 document.cookie 全量（含 web_session/a1/webId 等）。 */
export interface LoginResult {
  cookie: string;
  user_id?: string;
  /** 原本就已登录（未触发扫码即检测到）。 */
  alreadyLoggedIn?: boolean;
  /** 续期凭证（bili 扫码返回；bili 可自动保活）。 */
  refreshToken?: string;
}

/** 浏览器 profile 的登录 cookie 快照。null = 浏览器里没有登录标记，不等于确定登出。 */
export interface LoginCookieSnapshot {
  /** `k=v; k2=v2`（含 HttpOnly；按 key 排序，便于稳定比较）。 */
  cookie: string;
  user_id?: string;
}

/** 扫码登录能力（channel 级——平台账号登录，无需实例化 source）。 */
export interface Loginable {
  scanLogin(
    emitQr: (qr: string | null) => void,
    opts?: { timeoutMs?: number; signal?: AbortSignal },
  ): Promise<LoginResult>;
}

/** 翻页能力（可选能力）。 */
export interface Pageable {
  fetchMore(cursor?: string, ctx?: { existing: number }): Promise<FetchPage>;
}

/** 单条详情能力（可选）：列表 item 缺全文/多图时按需抓详情。 */
export interface ItemDetailSource {
  resolveDetail(item: Item, opts?: { passive?: boolean }): Promise<Item | null>;
}

/** 热搜词能力（可选）。 */
export interface HotWordSource {
  resolveHotWord(word: string): Promise<Item[]>;
}

/**
 * XML 时代的频道标题/链接选项。
 * @deprecated 渠道输出已是结构化数据；保留类型只为兼容旧渠道声明，值是死配置。
 */
export interface SerializeOptions {
  channelTitle?: string;
  channelLink?: string;
  total?: number;
}

/** 一次抓取的返回形状（翻页渠道可带真实总数）。 */
export interface FetchResultLike {
  items: Item[];
  total?: number;
}

/** api channel 的 fetch 装配：直接返回结构化结果。 */
export function apiFetch(
  fetchResult: () => Promise<FetchResultLike>,
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
