/**
 * naming 黄金集 —— 从旧 Rust 测试直译（含真实字幕组文件名回归）。
 * 用 bun test 跑：bun run --cwd packages/media-manager test
 */

import { describe, expect, test } from "bun:test";
import {
  canonicalFilename,
  parseEpisode,
  parseKind,
  seasonDir,
  showDir,
} from "./naming";

const se = (season: number, episode: number) => ({ season, episode });

describe("模式 1: SxxEyy", () => {
  test("标准形式与容忍变体", () => {
    const cases: [string, { season: number; episode: number }][] = [
      ["Game.of.Thrones.S01E01.1080p.mkv", se(1, 1)],
      ["Game of Thrones s01e02.mkv", se(1, 2)],
      ["Show.S1E1.mkv", se(1, 1)],
      ["Show.S01.E01.mkv", se(1, 1)],
      ["Show.S01 E01.mkv", se(1, 1)],
      ["Show.S12E103.mkv", se(12, 103)],
      ["[Group] Show - S03E07 [1080p].mkv", se(3, 7)],
    ];
    for (const [input, want] of cases)
      expect(parseEpisode(input)).toEqual(want);
  });
});

describe("模式 2: 1x01", () => {
  test("识别", () => {
    expect(parseEpisode("Game of Thrones 1x01.mkv")).toEqual(se(1, 1));
    expect(parseEpisode("Show.2x13.HDTV.mkv")).toEqual(se(2, 13));
    expect(parseEpisode("Show 10x05.mkv")).toEqual(se(10, 5));
  });
  test("分辨率里的 x 不是集号", () => {
    expect(parseEpisode("Show.1920x1080.mkv")).toBeUndefined();
    expect(parseEpisode("Show [1280x720].mkv")).toBeUndefined();
    expect(parseEpisode("Movie.3840x2160.HDR.mkv")).toBeUndefined();
  });
});

describe("模式 3: 第N集", () => {
  test("集/话/話/回", () => {
    expect(parseEpisode("权力的游戏 第01集.mkv")).toEqual(se(1, 1));
    expect(parseEpisode("进击的巨人 第25话.mkv")).toEqual(se(1, 25));
    expect(parseEpisode("某番 第 3 話.mkv")).toEqual(se(1, 3));
    expect(parseEpisode("剧名.第12回.mkv")).toEqual(se(1, 12));
  });
});

describe("模式 4: EP01", () => {
  test("识别", () => {
    expect(parseEpisode("[字幕组] 剧名 EP01 [1080p].mkv")).toEqual(se(1, 1));
    expect(parseEpisode("Show.E05.mkv")).toEqual(se(1, 5));
    expect(parseEpisode("Show ep.12.mkv")).toEqual(se(1, 12));
  });
  test("分辨率后缀不干扰", () => {
    expect(parseEpisode("Show 1080p.mkv")).toBeUndefined();
    expect(parseEpisode("Show.720p.WEB-DL.mkv")).toBeUndefined();
  });
});

describe("模式 5: [NN] 方括号裸集号", () => {
  test("真实字幕组命名", () => {
    const cases: [string, { season: number; episode: number }][] = [
      [
        "[A.I.R.nesSub][Ping_Pong_The_Animation][01][BDRIP][HEVC_FLACx2_THDx2_SUPx3].mkv",
        se(1, 1),
      ],
      [
        "[A.I.R.nesSub][Ping_Pong_The_Animation][11][BDRIP][HEVC_FLACx2_THDx2_SUPx3].mkv",
        se(1, 11),
      ],
      ["[Group][Show][03][1080p][FLAC].mkv", se(1, 3)],
      ["[字幕组][某番][12][BDRIP].mkv", se(1, 12)],
      ["[Nekomoe kissaten][Show][07][Ma10p_1080p].mkv", se(1, 7)],
    ];
    for (const [input, want] of cases)
      expect(parseEpisode(input)).toEqual(want);
  });
  test("集号后跟集标题（阿特拉斯字幕组）", () => {
    expect(
      parseEpisode(
        "[AtlasSubs][命运-奇异赝品_Fate／strange Fake][01_英灵事件][简繁日内封PGS][1080p HEVC AAC].mkv",
      ),
    ).toEqual(se(1, 1));
    expect(parseEpisode("[Group][Show][05 Title Here][1080p].mkv")).toEqual(
      se(1, 5),
    );
    expect(parseEpisode("[Group][Show][07.Title][1080p].mkv")).toEqual(
      se(1, 7),
    );
  });
  test("数字后必须是分隔符（排除规格组）", () => {
    expect(parseEpisode("[Group][Show][1080p HEVC AAC].mkv")).toBeUndefined();
    expect(parseEpisode("[Group][Show][10bit].mkv")).toBeUndefined();
  });
  test("年份与规格组不是集号", () => {
    expect(parseEpisode("[Group][Show][2014][BDRIP].mkv")).toBeUndefined();
    expect(parseEpisode("[Group][Show][HEVC_FLACx2].mkv")).toBeUndefined();
    expect(parseEpisode("[Group][Show][1080p].mkv")).toBeUndefined();
  });
  test("多组数字取最后一个", () => {
    expect(parseEpisode("[Group][Show S02][02][07][BDRIP].mkv")).toEqual(
      se(1, 7),
    );
  });
});

describe("模式 6: 「 - NN 」", () => {
  test("短横两侧必须有空格", () => {
    expect(parseEpisode("[Sakurato] 剧名 - 01 [1080p].mkv")).toEqual(se(1, 1));
    expect(parseEpisode("Show - 5.mkv")).toEqual(se(1, 5));
    expect(parseEpisode("Show-SP01.mkv")).toBeUndefined();
    expect(parseEpisode("Show.1080-2011.mkv")).toBeUndefined();
  });
});

describe("特典识别", () => {
  test("[01][Preview] 不与真第 1 集撞号", () => {
    const real =
      "[A.I.R.nesSub][Ping_Pong_The_Animation][01][BDRIP][HEVC_FLACx2_THDx2_SUPx3].mkv";
    const preview =
      "[A.I.R.nesSub][Ping_Pong_The_Animation][01][Preview][BDRIP][HEVC_THD_SUP].mkv";
    expect(parseEpisode(real)).toEqual(se(1, 1));
    expect(parseEpisode(preview)).toBeUndefined();
    expect(parseKind(preview)?.kind).toBe("special");
  });
  test("[00_SP] 与 CM 组是特典", () => {
    const f = "[AtlasSubs][命运-奇异赝品][00_SP][简繁日内封PGS].mkv";
    expect(parseKind(f)?.kind).toBe("special");
    expect(
      parseKind("[A.I.R.nesSub][Ping_Pong][CompleteBOX_CM1][BDRIP].mkv")?.kind,
    ).toBe("special");
  });
  test("关键字识别", () => {
    for (const c of ["[NCOP]", "[特典]", "[Trailer]", "[予告]", "[Menu]"]) {
      expect(parseKind(`[Group][Show]${c}[BDRIP].mkv`)?.kind).toBe("special");
    }
  });
  test("CM 提示号", () => {
    const k = parseKind("[Group][Show][CompleteBOX_CM3][BDRIP].mkv");
    expect(k).toEqual({ kind: "special", hint: 3 });
  });
  test("不误伤剧名", () => {
    expect(
      parseEpisode("[VCB-Studio] Spice and Wolf [01][Ma10p_1080p].mkv"),
    ).toEqual(se(1, 1));
  });
});

describe("不猜测", () => {
  test("无模式返回 undefined", () => {
    for (const input of [
      "权力的游戏.mkv",
      "Some.Movie.2011.1080p.BluRay.mkv",
      "剧名 合集.mkv",
      "",
      "Show 2011-04-17.mkv",
    ]) {
      expect(parseEpisode(input)).toBeUndefined();
    }
  });
  test("目录名里的季集号不干扰", () => {
    expect(parseEpisode("/media/Show.S01E01/剧名.mkv")).toBeUndefined();
    expect(parseEpisode("/media/Show.S01/Show.S01E01.mkv")).toEqual(se(1, 1));
  });
  test("SxxEyy 优先级最高", () => {
    expect(parseEpisode("剧名 第9集 S02E05.mkv")).toEqual(se(2, 5));
  });
});

describe("命名生成", () => {
  test("canonicalFilename 只保留扩展名", () => {
    expect(canonicalFilename("[字幕组] 剧名 第1集 [1080p].mkv", se(1, 1))).toBe(
      "S01E01.mkv",
    );
    expect(canonicalFilename("Show.S01E01.mp4", se(1, 1))).toBe("S01E01.mp4");
    expect(canonicalFilename("Show.S01E01", se(1, 1))).toBe("S01E01.mkv");
  });
  test("seasonDir/showDir", () => {
    expect(seasonDir(0)).toBe("Season 00");
    expect(seasonDir(12)).toBe("Season 12");
    expect(showDir("权力的游戏", "2011")).toBe("权力的游戏 (2011)");
    expect(showDir("权力的游戏")).toBe("权力的游戏");
    expect(showDir("A/B: C", "2011")).toBe("A_B_ C (2011)");
    expect(showDir("///")).toBe("___");
    expect(showDir("   ")).toBe("未命名");
  });
});
