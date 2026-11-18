/**
 * 数据服务进程生命周期集成测试（真实 spawn tauri-rss 的 service.js）。
 * 无 bun / service.js 时整组跳过（组件入口的定位与错误路径由宿主 smoke 覆盖）。
 */

import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  locateServiceJs,
  startRssService,
  type RssServiceHandle,
} from "./entry";

// 优先 AMOEBA_SERVICE_JS / 已构建的组件包（bun run --cwd packages/rss-service pack）
const serviceJs =
  locateServiceJs(join(import.meta.dir, "..", "out", "component")) ?? "";
const run = serviceJs.length > 0;

const tmpRoot = await mkdtemp(join(tmpdir(), "rss-service-test-"));
let handle: RssServiceHandle | undefined;

afterAll(async () => {
  await handle?.service.stop().catch(() => undefined);
  await rm(tmpRoot, { recursive: true, force: true });
});

describe.skipIf(!run)("startRssService（真实服务进程）", () => {
  test(
    "spawn → READY → health → stop 全链路",
    async () => {
      handle = await startRssService({
        bun: "bun",
        serviceJs,
        dataDir: join(tmpRoot, "data"),
        parentPid: process.pid,
        log: () => undefined,
      });

      const info = handle.service.endpoint();
      expect(info.port).toBeGreaterThan(0);
      expect(info.baseUrl).toBe(`http://127.0.0.1:${info.port}`);
      expect(info.token.length).toBeGreaterThan(0);

      const health = await handle.service.health();
      expect(health.ok).toBe(true);
      expect(health.rev).toBe(0);

      await handle.service.stop();
      expect(handle.proc.exitCode).not.toBeNull();
      // stop 幂等
      await handle.service.stop();
    },
    30_000,
  );

  test("停止后 health 拒绝连接（服务确已退出）", async () => {
    const h = handle;
    if (!h) return;
    await expect(h.service.health()).rejects.toThrow();
  });
});
