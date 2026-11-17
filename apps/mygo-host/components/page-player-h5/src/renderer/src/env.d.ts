/** vite 的 css side-effect import 需要 TS 模块声明；页面 bridge 的全局类型也在此。 */

declare module "*.css";

/** mygo 注入的运行时（本页只用 call/on） */
interface MygoGlobal {
  call(method: string, ...args: unknown[]): Promise<unknown>;
  on(name: string, listener: (payload: unknown) => void): () => void;
}

declare global {
  interface Window {
    /** mygo 注入（trusted page 可用）；window.tab 由 @host/sdk 的全局声明提供 */
    mygo?: MygoGlobal;
  }
}
