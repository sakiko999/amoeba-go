/**
 * Bangumi 映射测试：全部用真实 API 响应 fixture（crates/core 黄金集直传），
 * 不发网络请求。网络层由 `--probe-bangumi` 探针验证。
 */

import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { mapEpisodes, mapSearch, mapSubject } from "./provider";
import { ProviderError } from "@tabs/platform-kit/provider";

const fixture = (name: string): unknown =>
  JSON.parse(
    readFileSync(
      new URL(`../tests/fixtures/${name}`, import.meta.url),
      "utf8",
    ),
  );

describe("bangumi", () => {
  test("mapSubject 提取中文标题/评分/工作室", () => {
    const m = mapSubject(fixture("bangumi_subject_8.json"), "8", "tv");
    expect(m.title).toBe("Code Geass 反叛的鲁路修R2");
    expect(m.originalTitle).toBe("コードギアス 反逆のルルーシュR2");
    expect(m.premiered).toBe("2008-04-06");
    expect(m.studios).toEqual(["サンライズ"]);
    expect(m.ratings).toEqual([
      { source: "bangumi", value: 8.3, votes: 18749 },
    ]);
    expect(m.uniqueIds).toEqual([{ source: "bangumi", value: "8" }]);
    expect(m.episodeCount).toBe(25);
    expect(m.seasonCount).toBe(1);
    expect(m.posterUrl).toContain("lain.bgm.tv");
  });

  test("mapSubject 拒绝书籍/音乐（type 1/3）", () => {
    expect(() => mapSubject({ type: 1, name: "x" }, "1", "tv")).toThrow(
      ProviderError,
    );
    expect(() => mapSubject({ type: 3, name: "x" }, "3", "tv")).toThrow(
      ProviderError,
    );
  });

  test("name 与 name_cn 相同 → 取 infobox 别名", () => {
    const raw = {
      type: 2,
      name: "同名",
      name_cn: "同名",
      infobox: [{ key: "别名", value: [{ v: "别名A" }] }],
    };
    expect(mapSubject(raw, "1", "tv").title).toBe("别名A");
  });

  test("mapSearch 取中文名并限量", () => {
    const raw = fixture("bangumi_search.json");
    const all = mapSearch(raw, 100);
    expect(all.length).toBe(5);
    expect(all[0]).toMatchObject({
      id: "402128",
      title: "命运-奇异赝品 黎明低语",
      year: "2023",
      mediaType: "tv",
    });
    expect(mapSearch(raw, 2)).toHaveLength(2);
  });

  test("mapEpisodes 整数集号、秒转分钟、按集排序", () => {
    const eps = mapEpisodes(fixture("bangumi_episodes_8.json"), "8");
    expect(eps).toHaveLength(25);
    expect(eps[0]).toMatchObject({
      season: 1,
      episode: 1,
      title: "魔王的苏醒之日",
      originalTitle: "魔神 が 目覚める 日",
      aired: "2008-04-06",
      runtimeMinutes: 24,
    });
    expect(eps[0]?.uniqueIds).toContainEqual({
      source: "bangumi-episode",
      value: "522",
    });
  });

  test("mapEpisodes 跳过 0 与小数集号", () => {
    const raw = {
      data: [
        { ep: 0, name: "op" },
        { ep: 1.5, name: "sp" },
        { ep: 2, sort: 2, name_cn: "第二集" },
        { ep: 1, name: "第一集" },
      ],
    };
    const eps = mapEpisodes(raw, "1");
    expect(eps.map((e) => e.episode)).toEqual([1, 2]);
    expect(eps.map((e) => e.title)).toEqual(["第一集", "第二集"]);
  });
});
