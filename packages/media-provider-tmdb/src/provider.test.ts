/**
 * TMDB 映射测试：fixture 来自真实 API 响应（原 Rust 测试黄金集）。
 */

import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import {
  mapMovieDetail,
  mapMovieSearch,
  mapSeasonEpisodes,
  mapTvDetail,
  mapTvSearch,
} from "./provider";

const fixture = (name: string): unknown =>
  JSON.parse(
    readFileSync(
      new URL(`../tests/fixtures/${name}`, import.meta.url),
      "utf8",
    ),
  );

describe("tmdb", () => {
  test("mapTvDetail 提取标题/评分/演职员/外部 ID", () => {
    const m = mapTvDetail(fixture("tmdb_tv_1399.json"), "1399");
    expect(m.title).toBe("权力的游戏");
    expect(m.originalTitle).toBe("Game of Thrones");
    expect(m.premiered).toBe("2011-04-17");
    expect(m.status).toBe("Ended");
    expect(m.genres).toHaveLength(3);
    expect(m.studios).toContain("HBO");
    expect(m.ratings).toEqual([
      { source: "themoviedb", value: 8.471, votes: 27708 },
    ]);
    expect(m.uniqueIds).toEqual([
      { source: "tmdb", value: "1399" },
      { source: "imdb", value: "tt0944947" },
    ]);
    expect(m.actors[0]).toMatchObject({
      name: "彼特·丁拉基",
      role: "Tyrion 'The Halfman' Lannister",
      order: 0,
    });
    expect(m.seasonCount).toBe(8);
    expect(m.episodeCount).toBe(73);
    expect(m.posterUrl).toContain("image.tmdb.org");
  });

  test("mapMovieDetail 提取电影字段", () => {
    const m = mapMovieDetail(fixture("tmdb_movie_550.json"), "550");
    expect(m.mediaType).toBe("movie");
    expect(m.title).toBe("搏击俱乐部");
    expect(m.originalTitle).toBe("Fight Club");
    expect(m.premiered).toBe("1999-10-15");
    expect(m.ratings[0]?.value).toBe(8.437);
    expect(m.uniqueIds).toContainEqual({
      source: "imdb",
      value: "tt0137523",
    });
  });

  test("mapTvSearch / mapMovieSearch 限量并带年份", () => {
    const tv = mapTvSearch(fixture("tmdb_search_tv.json"), 20);
    expect(tv).toHaveLength(1);
    expect(tv[0]).toMatchObject({
      id: "229858",
      title: "命运／奇异赝品",
      year: "2024",
      mediaType: "tv",
    });
    expect(mapTvSearch(fixture("tmdb_search_tv.json"), 0)).toHaveLength(0);
    expect(mapMovieSearch({ results: [] }, 20)).toEqual([]);
  });

  test("mapSeasonEpisodes 解析集号/时长/剧照", () => {
    const eps = mapSeasonEpisodes(fixture("tmdb_season_1399_s1.json"), 1);
    expect(eps).toHaveLength(10);
    expect(eps[0]).toMatchObject({
      season: 1,
      episode: 1,
      title: "凛冬将至",
      aired: "2011-04-17",
      runtimeMinutes: 62,
    });
    expect(eps[0]?.thumbUrl).toContain("image.tmdb.org");
  });
});
