/**
 * 宿主 SDK：tab 子包、宿主（apps/desktop）与 runtime 页面包共享的唯一契约。
 * 包含 tab 契约（TabManifest / TabApi / HostApi）、页面包契约（PageManifest / main 扩展上下文）
 * 以及宿主全局状态（HostState / HOST_STATE_CHANNELS）。纯类型 + 少量运行时工具，零依赖。
 * 刻意不引用 electron 类型（本包不安装 electron）：getWindow 返回 unknown，扩展自行断言。
 */

/* ---------- 宿主状态（host 页面、tab 与 runtime 页面订阅的主题等全局状态） ---------- */

export type Theme = "dark" | "light";

/** 宿主唯一状态源；新增全局状态时在这里加字段（各方按整份状态订阅） */
export interface HostState {
  theme: Theme;
}

export type HostStatePatch = Partial<HostState>;

/** 宿主与各 preload 之间的固定 IPC 渠道名 */
export const HOST_STATE_CHANNELS = {
  /** sendSync：preload 首帧同步取初始状态 */
  sync: "host:state:sync",
  /** invoke：随时取当前状态 */
  get: "host:state:get",
  /** 只有宿主 UI 能写；tab 与页面只读 */
  set: "host:state:set",
  /** 主进程 → 所有 webContents 的状态变更广播 */
  changed: "host:state",
} as const;

/* ---------- runtime 页面包（page.json + 可选 main 扩展） ---------- */

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

/* ---------- tab（编译期子包与 runtime 页面共用的实例契约） ---------- */

/** 一个 tab 种类的静态描述，由每个 tab 子包导出一份。 */
export interface TabManifest {
  /** 唯一 id，同时是构建产物目录名（目录名约定 packages/tab-<id>） */
  id: string;
  /** tab 条上的默认标题 */
  title: string;
  /** tab 条上圆点的强调色 */
  color: string;
}

/** 主进程视角下一个活着的 tab 实例。 */
export interface TabSummary {
  instanceId: string;
  kind: string;
  title: string;
  active: boolean;
  /** 页面 did-finish-load 前为 true：view 尚不显示，host 渲染过渡页 */
  loading?: boolean;
}

/** 主进程聚合的运行环境信息（renderer 无法自取，必须走 IPC）。 */
export interface SystemInfo {
  platform: string;
  arch: string;
  cpus: number;
  memoryGb: number;
  electron: string;
  chrome: string;
  node: string;
}

/** 每个 tab 页面里 window.tab 的形状（由 preload 注入）。 */
export interface TabApi {
  readonly instanceId: string;
  readonly kind: string;
  /** 更新 tab 条上本 tab 的标题（如 Notes 页同步字数） */
  setTitle(title: string): void;
  close(): void;
  /** IPC 往返演示，resolve "pong:<value> @ <kind>" */
  ping(value: string): Promise<string>;
  systemInfo(): Promise<SystemInfo>;
  /** 宿主全局状态的当前快照 */
  state(): Promise<HostState>;
  /** 订阅整份宿主状态（主题、后续新增字段） */
  onStateChanged(cb: (state: HostState) => void): void;
  /** 兼容旧 tab：等价于 state().theme */
  theme(): Promise<Theme>;
  /** 兼容旧 tab：等价于 onStateChanged 的 theme 切片 */
  onThemeChanged(cb: (theme: Theme) => void): void;
}

/** host（tab 条页面）里 window.host 的形状。 */
export interface HostApi {
  listKinds(): Promise<TabManifest[]>;
  createTab(kind: string): Promise<void>;
  /** 请主进程弹原生“新建 tab”菜单（host DOM 会被 tab 视图遮挡，DOM 菜单不可行） */
  openNewTabMenu(): void;
  /** 请主进程在某 tab 上弹原生右键菜单（关闭 / 关闭其他 / 关闭右侧） */
  openTabContextMenu(instanceId: string): void;
  /** 切换亮/暗主题（主进程广播 host:state 给 host、tab 与 runtime 页面） */
  toggleTheme(): void;
  /** 请主进程弹原生调试菜单（主进程 inspector / 启动器界面 / 当前 tab 的 DevTools） */
  openDebugMenu(): void;
  state(): Promise<HostState>;
  onStateChanged(cb: (state: HostState) => void): void;
  /** 兼容旧接口：等价于 state().theme */
  theme(): Promise<Theme>;
  /** 兼容旧接口：等价于 onStateChanged 的 theme 切片 */
  onThemeChanged(cb: (theme: Theme) => void): void;
  activateTab(instanceId: string): void;
  closeTab(instanceId: string): void;
  /** 拖拽排序：把 instanceId 移到 overInstanceId 当前的位置 */
  moveTab(instanceId: string, overInstanceId: string): void;
  /** 主进程每次 tab 状态变化都会回调；host 页面只注册一次，无 unsubscribe（demo 取舍） */
  onTabsChanged(cb: (tabs: TabSummary[]) => void): void;
  /** runtime 页面安装/卸载/重载后重新拉取 kinds（tab 圆点颜色） */
  onKindsChanged(cb: () => void): void;
}

/** preload 启动时 sendSync("app:whoami") 的返回值；unknown 一律不注入 bridge。 */
export type Whoami =
  | { role: "host" }
  | { role: "tab"; instanceId: string; kind: string }
  | { role: "unknown" };

declare global {
  interface Window {
    /** 仅 host 页面存在 */
    host?: HostApi;
    /** 仅 tab 页面存在 */
    tab?: TabApi;
  }
}

/** tab renderer 里取 bridge；缺失说明 preload 没跑（比如直接用浏览器打开了页面）。 */
export function tabApi(): TabApi {
  const api = window.tab;
  if (!api) throw new Error("window.tab is missing — preload did not run");
  return api;
}

/** host renderer 里取 bridge。 */
export function hostApi(): HostApi {
  const api = window.host;
  if (!api) throw new Error("window.host is missing — preload did not run");
  return api;
}

/** renderer 里安全取 DOM 元素：html 骨架是写死的，取不到属于模板错误，直接抛。 */
export function dom<T extends HTMLElement>(selector: string): T {
  const found = document.querySelector<T>(selector);
  if (!found) throw new Error(`element not found: ${selector}`);
  return found;
}
