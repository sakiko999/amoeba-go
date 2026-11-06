/**
 * Tab SDK：shell（apps/desktop）与各 tab 子包之间的唯一契约。
 * 纯类型 + 几个运行时小工具；HostState 类型来自 @pages/sdk（页面与 tab 共用）。
 */

import type { HostState, Theme } from "@pages/sdk";

export type { HostState, HostStatePatch, Theme } from "@pages/sdk";

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

/* 主题（Theme）与 HostState 由 @pages/sdk 定义并 re-export。 */

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
  /** 切换亮/暗主题（主进程广播 theme:changed 给 host 与所有 tab） */
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
