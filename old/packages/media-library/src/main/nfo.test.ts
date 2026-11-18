/**
 * NFO 写入 / 解析往返测试，覆盖 Kodi 的硬性约束：
 * uniqueid 有且只有一个 default、ratings name 不能是 default、单集用 aired。
 */

import { describe, expect, test } from "bun:test";
import type { EpisodeMetadata, MediaMetadata } from "../shared/model";
import { newMetadata } from "../shared/model";
import { parseEpisodeNfo, parseMetadataNfo } from "./nfo";
import { writeEpisodeNfo, writeMovieNfo, writeShowNfo } from "./nfoWriter";

function sampleMeta(): MediaMetadata {
  const m = newMetadata("tv", "测试 & <剧>");
  m.originalTitle = "Test Show";
  m.plot = '第一行\n第二行 "引号" & <尖括号>';
  m.premiered = "2011-04-17";
  m.status = "Ended";
  m.genres = ["剧情", "奇幻"];
  m.studios = ["HBO"];
  m.ratings = [
    { source: "themoviedb", value: 8.5, votes: 100 },
    { source: "imdb", value: 8 },
  ];
  m.uniqueIds = [
    { source: "tmdb", value: "1399" },
    { source: "imdb", value: "tt0944947" },
  ];
  m.actors = [
    { name: "演员甲", role: "角色甲", order: 0 },
    { name: "演员乙", order: 1, thumbUrl: "https://example.com/a.jpg" },
  ];
  m.seasonCount = 8;
  m.episodeCount = 73;
  return m;
}

describe("nfo", () => {
  test("作品级 NFO 往返不丢字段", () => {
    const parsed = parseMetadataNfo(writeShowNfo(sampleMeta()), "tv");
    expect(parsed).toEqual(sampleMeta());
  });

  test("uniqueid 只有一个 default=true；ratings name 不为 default", () => {
    const xml = writeShowNfo(sampleMeta());
    const ids = [...xml.matchAll(/<uniqueid[^>]*default="(true|false)"/g)].map(
      (m) => m[1],
    );
    expect(ids.filter((v) => v === "true")).toHaveLength(1);
    expect(ids[0]).toBe("true");
    expect(xml).toContain('<rating name="themoviedb" max="10" default="true">');
    expect(xml).not.toContain('name="default"');
  });

  test("空字符串不写标签", () => {
    const m = newMetadata("movie", "只有标题");
    const xml = writeMovieNfo(m);
    expect(xml).toContain("<title>只有标题</title>");
    expect(xml).not.toContain("<plot>");
    expect(xml).not.toContain("<studio>");
  });

  test("单集 NFO 用 aired 且 runtime 为分钟", () => {
    const ep: EpisodeMetadata = {
      season: 1,
      episode: 2,
      title: "第二集",
      aired: "2011-04-24",
      runtimeMinutes: 56,
      actors: [],
      uniqueIds: [],
    };
    const xml = writeEpisodeNfo(ep);
    expect(xml).toContain("<aired>2011-04-24</aired>");
    expect(xml).toContain("<runtime>56</runtime>");
    expect(xml).not.toContain("<premiered>");
    const light = parseEpisodeNfo(xml);
    expect(light).toEqual({
      title: "第二集",
      aired: "2011-04-24",
      runtimeMinutes: 56,
    });
  });

  test("兼容老式平铺 <rating> 与 CRLF 规范化", () => {
    const xml =
      "<movie><title>旧文件</title><rating>7.5</rating><plot>a\r\nb</plot></movie>";
    const parsed = parseMetadataNfo(xml, "movie");
    expect(parsed?.ratings).toEqual([{ source: "imdb", value: 7.5 }]);
    expect(parsed?.plot).toBe("a\nb");
  });
});
