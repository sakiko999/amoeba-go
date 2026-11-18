/** bun test 预加载：把 electron 替换成最小桩（不发网络；缓存落临时目录）。 */

import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { mock } from "bun:test";

const userData = mkdtempSync(join(tmpdir(), "platform-kit-test-"));

mock.module("electron", () => ({
  app: { getPath: () => userData },
  net: {
    fetch: () => {
      throw new Error("network is disabled in unit tests");
    },
  },
}));
