/** bun test 预加载：把 electron 替换成最小桩（扩展测试不发网络）。 */

import { mock } from "bun:test";

mock.module("electron", () => ({
  app: { getPath: () => process.cwd() },
  net: {
    fetch: () => {
      throw new Error("network is disabled in unit tests");
    },
  },
}));
