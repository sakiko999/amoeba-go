/**
 * 豆瓣映射测试（只支持按 ID 抓取，无搜索）。fixture 为真实 API 响应。
 */

import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { mapSubject } from "./provider";

const fixture = (name: string): unknown =>
  JSON.parse(
    readFileSync(
      new URL(`../tests/fixtures/${name}`, import.meta.url),
      "utf8",
    ),
  );

describe("douban", () => {
  test("mapSubject 剧集：导演在前、演员在后", () => {
    const m = mapSubject(fixture("douban_tv_3016187.json"), "3016187");
    expect(m.mediaType).toBe("tv");
    expect(m.title).toBe("权力的游戏 第一季");
    expect(m.originalTitle).toBe("Game of Thrones Season 1");
    expect(m.genres).toEqual(["剧情", "奇幻", "冒险"]);
    expect(m.ratings).toEqual([
      { source: "douban", value: 9.5, votes: 505366 },
    ]);
    expect(m.uniqueIds).toEqual([{ source: "douban", value: "3016187" }]);
    expect(m.actors[0]).toEqual({
      name: "蒂莫西·范·帕腾",
      role: "导演",
      order: 0,
    });
    expect(m.actors[4]).toMatchObject({ name: "肖恩·宾", order: 4 });
    expect(m.episodeCount).toBe(10);
    expect(m.seasonCount).toBe(1);
    expect(m.posterUrl).toContain("doubanio.com");
  });

  test("mapSubject 电影：release_date 缺失时不写 premiered", () => {
    const m = mapSubject(fixture("douban_movie_1292052.json"), "1292052");
    expect(m.mediaType).toBe("movie");
    expect(m.title).toBe("肖申克的救赎");
    expect(m.originalTitle).toBe("The Shawshank Redemption");
    expect(m.premiered).toBeUndefined();
    expect(m.episodeCount).toBeUndefined();
  });

  test("mapSubject 缺标题时抛错", () => {
    expect(() => mapSubject({ id: "1" }, "1")).toThrow();
  });
});
