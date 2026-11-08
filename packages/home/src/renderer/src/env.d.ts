/** vite 的 css side-effect import 需要 TS 模块声明；页面 bridge 的全局类型也在此。 */

import type {
  ComponentInfo,
  HostStateBridge,
  NavEntry,
} from "@host/sdk";

declare module "*.css";

declare global {
  interface Window {
    /** 宿主 pageBridge 注入（sendSync 初始状态 + 变更订阅） */
    hostState?: HostStateBridge;
    /** 本页 preload 注入（见 src/preload/index.ts） */
    homeNav?: {
      list(): Promise<NavEntry[]>;
      open(id: string): Promise<boolean>;
      components(): Promise<ComponentInfo[]>;
      onComponentsChanged(cb: () => void): void;
    };
  }
}

export {};
