/**
 * 数据渠道契约：渠道描述、实例化参数、抓取/翻页结果与可选能力接口。
 *
 * 纯类型、零运行时依赖；platform-* 渠道实现与数据消费方共用同一份定义。
 * 装配工厂（apiFetch / apiFetchMore / makeApiChannel …）留在
 * @tabs/platform-kit/data，这里只放契约。
 */

import type { Item, Kind } from "./feed";

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
