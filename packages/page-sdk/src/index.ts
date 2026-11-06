/**
 * 页面包契约：宿主（apps/desktop）与页面 main 扩展共享的纯类型，零依赖。
 * 刻意不引用 electron 类型（page-sdk 自身不安装 electron）；getWindow 返回 unknown，
 * 页面扩展按需断言成 BrowserWindow。
 */

/* ---------- 宿主状态（页面 preload 订阅的主题等全局状态） ---------- */

export type Theme = "dark" | "light";

/** 宿主唯一状态源；新增全局状态时在这里加字段（页面按整份状态订阅） */
export interface HostState {
  theme: Theme;
}

export type HostStatePatch = Partial<HostState>;

/** 宿主与页面 preload 之间的固定 IPC 渠道名 */
export const HOST_STATE_CHANNELS = {
  /** sendSync：preload 首帧同步取初始状态 */
  sync: "host:state:sync",
  /** invoke：随时取当前状态 */
  get: "host:state:get",
  /** 只有宿主 UI 能写；页面只读 */
  set: "host:state:set",
  /** 主进程 → 所有 webContents 的状态变更广播 */
  changed: "host:state",
} as const;

/** 页面包根目录下的 page.json */
export interface PageManifest {
  /** URL host 用，必须是小写 DNS label：[a-z0-9-] */
  id: string;
  name: string;
  version?: string;
  /** tab 圆点颜色，如 "#e0a458" */
  color?: string;
  /** 相对包根的入口 HTML */
  entry: string;
  /** 相对包根的 sandbox CJS preload */
  preload?: string;
  /** 相对包根的 CJS 主进程扩展 */
  main?: string;
}

export type PageSource = "builtin" | "external";

/** 宿主视角下的页面状态 */
export interface PageSummary {
  id: string;
  name: string;
  color: string;
  source: PageSource;
  /** active = main 扩展已激活，可以创建 tab */
  state: "inactive" | "active" | "error";
  hasMain: boolean;
  error?: string;
}

/** 页面 main 扩展拿到的 IPC 事件（结构上是 Electron 事件的最小子集） */
export interface PageIpcEvent {
  sender: { id: number };
  senderFrame?: { url: string } | null;
}

export type PageIpcHandler = (
  event: PageIpcEvent,
  ...args: unknown[]
) => unknown;

/** 宿主注入的 IPC 注册器：卸载/重载时宿主会清理这里登记的全部 channel */
export interface PageIpc {
  handle(channel: string, listener: PageIpcHandler): void;
  on(channel: string, listener: PageIpcHandler): void;
}

/** 页面 main 扩展的 activate 上下文 */
export interface PageMainContext {
  readonly manifest: Readonly<PageManifest>;
  /** 包根绝对路径 */
  readonly pageRoot: string;
  /** 宿主为该页面准备的持久数据目录（userData/page-data/<id>） */
  readonly dataDir: string;
  /** 宿主 app 路径 */
  readonly appPath: string;
  readonly ipc: PageIpc;
  /** BrowserWindow；页面扩展自行断言类型 */
  getWindow(): unknown;
  log(...args: unknown[]): void;
}

/** main.js（CJS）必须导出 activate；deactivate 可选 */
export interface PageMainModule {
  activate(ctx: PageMainContext): void | Promise<void>;
  deactivate?(): void | Promise<void>;
}
