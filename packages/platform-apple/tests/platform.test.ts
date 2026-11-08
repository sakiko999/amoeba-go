import { describe, expect, test } from "bun:test";
import { channels } from "../src/channels.ts";

describe("渠道声明", () => {
  test("非空且 key 唯一", () => {
    expect(channels.length).toBeGreaterThan(0);
    expect(new Set(channels.map((c) => c.key)).size).toBe(channels.length);
    for (const c of channels) expect(c.name.length).toBeGreaterThan(0);
  });

  test("声明了 defaultInfo 的渠道可零参实例化", () => {
    for (const c of channels) {
      if (c.defaultInfo === undefined) continue;
      const source = c.getSource({ ...c.defaultInfo });
      expect(typeof source.fetch).toBe("function");
    }
  });

  test("passive 渠道必须提供 fetchPassive", () => {
    for (const c of channels.filter((x) => x.passive === true)) {
      expect(typeof c.getSource({}).fetchPassive).toBe("function");
    }
  });
});
