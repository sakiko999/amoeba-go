/** vite 的 css side-effect import 需要 TS 模块声明；页面 bridge 的全局类型也在此。 */

import type { HostStateBridge } from "@host/sdk";

declare module "*.css";

/** mygo 注入的运行时（api.ts 只用 call/channel 两个成员） */
interface MygoGlobal {
  call(method: string, ...args: unknown[]): Promise<unknown>;
  channel<T>(onmessage?: (value: T) => void): { close(): void };
  on(name: string, listener: (payload: unknown) => void): () => void;
}

declare global {
  interface Window {
    /** mygo 注入（trusted page 可用） */
    mygo?: MygoGlobal;
    /** 宿主 pageBridge 注入（注入时刻同步初始主题 + 变更订阅） */
    hostState?: HostStateBridge;
  }
}
