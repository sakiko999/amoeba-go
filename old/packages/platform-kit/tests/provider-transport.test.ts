/** provider 传输统一：appHost.http 存在时必须走它（与数据渠道同一条路径）。 */

import { describe, expect, test } from "bun:test";

import { getJsonCached } from "../src/provider/http";

describe("provider 传输", () => {
  test("优先走 appHost.http（宿主形态统一传输面）", async () => {
    const seen: string[] = [];
    const g = globalThis as unknown as { appHost?: unknown };
    const prev = g.appHost;
    g.appHost = {
      http: {
        request: async (req: { url: string }) => {
          seen.push(req.url);
          return { status: 200, headers: {}, body: JSON.stringify({ ok: 1 }) };
        },
      },
    };
    try {
      const r = (await getJsonCached(
        "unit",
        ["transport", "apphost-first"],
        "http://unit.test/x",
      )) as { ok: number };
      expect(r).toEqual({ ok: 1 });
      expect(seen).toEqual(["http://unit.test/x"]);
    } finally {
      g.appHost = prev;
    }
  });
});
