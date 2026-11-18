/**
 * mygo 桥适配层：把宿主 Tabs 服务/事件包装成 Electron 版 hostApi 同形状的接口，
 * tab 条组件只依赖这里的类型与函数。桥的底层调用形式集中在 callTabs/onEvent 两个函数，
 * mygo 桥协议若有出入只改这里。
 */

export type Theme = "dark" | "light";

export interface TabSummary {
  instanceId: string;
  kind: string;
  title: string;
  active: boolean;
  loading: boolean;
}

export interface TabManifest {
  id: string;
  title: string;
  color: string;
}

export interface HostState {
  theme: Theme;
}

/** webview 页面拿不到 --mygo-titlebar-* CSS 变量，标题栏度量由宿主提供 */
export interface ChromeInfo {
  insetLeft: number;
  insetRight: number;
  height: number;
  bottom: number;
}

interface MygoBridge {
  call(method: string, ...args: unknown[]): Promise<unknown>;
  on(name: string, listener: (payload: unknown) => void): () => void;
  once(name: string, listener: (payload: unknown) => void): () => void;
}

declare global {
  interface Window {
    mygo?: MygoBridge;
  }
}

function bridge(): MygoBridge {
  if (window.mygo === undefined) throw new Error("window.mygo 未注入（页面不是 trusted page？）");
  return window.mygo;
}

async function callTabs<T>(method: string, ...args: unknown[]): Promise<T> {
  return (await bridge().call(`Tabs.${method}`, ...args)) as T;
}

function onEvent<T>(name: string, cb: (payload: T) => void): () => void {
  return bridge().on(name, (payload) => cb(payload as T));
}

export function hostApi() {
  return {
    onTabsChanged(cb: (tabs: TabSummary[]) => void): () => void {
      return onEvent<TabSummary[]>("tabs.changed", cb);
    },
    listKinds(): Promise<TabManifest[]> {
      return callTabs<TabManifest[]>("Kinds");
    },
    activateTab(instanceId: string): void {
      void callTabs("Activate", instanceId);
    },
    /** “+” 与 tab 右键菜单由宿主原生弹出：44px 的 chrome 视口放不下 DOM 浮层 */
    openNewTabMenu(): void {
      void callTabs("PopupNewTabMenu");
    },
    openTabContextMenu(instanceId: string): void {
      void callTabs("PopupTabMenu", instanceId);
    },
    closeTab(instanceId: string): void {
      void callTabs("Close", instanceId);
    },
    moveTab(instanceId: string, overInstanceId: string): void {
      void callTabs("Move", instanceId, overInstanceId);
    },
    toggleTheme(): void {
      void callTabs("ToggleTheme");
    },
    state(): Promise<HostState> {
      return callTabs<HostState>("State");
    },
    chrome(): Promise<ChromeInfo> {
      return callTabs<ChromeInfo>("Chrome");
    },
    onStateChanged(cb: (state: HostState) => void): () => void {
      return onEvent<HostState>("host.state", cb);
    },
  };
}

export type HostApi = ReturnType<typeof hostApi>;
