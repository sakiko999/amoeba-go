/**
 * bun test 预加载：把 electron 替换成最小桩。
 * 单元测试只跑纯映射函数（provider 的 map*、NFO 往返、naming），
 * 网络与文件写盘由 `--probe-bangumi` 探针在真实 Electron 进程内验证。
 */

import { mock } from "bun:test";

mock.module("electron", () => ({
  app: { getPath: () => process.cwd() },
  net: {
    fetch: () => {
      throw new Error("network is disabled in unit tests");
    },
  },
  // provider 测试会经公共入口（index）间接加载 service.ts，这里补齐它用到的面
  dialog: {
    showOpenDialog: async () => ({ canceled: true, filePaths: [] }),
  },
  shell: { showItemInFolder: () => {} },
}));
