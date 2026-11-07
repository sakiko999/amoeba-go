/**
 * 免窗口网络探针（开发用）：`electron . --probe-bangumi <id> [keyword]`。
 * 走主进程 net.fetch（系统证书），用于验证 Bangumi 免 token 渠道的完整链路：
 * 搜索 → 抓取 → 每集 → 缓存命中。`--probe-bangumi-apply <dir> <id>` 额外写盘
 * （只应对可丢弃目录执行）。
 */

import { readdir } from "node:fs/promises";
import {
  ProviderRegistry,
  scrapeApply,
  scrapePreview,
} from "@tabs/media-library";
import {
  createBangumiProvider,
  fetchBangumi,
  fetchBangumiEpisodes,
  searchBangumi,
} from "./provider";

/** 探针自带注册表：bangumi 是它唯一渠道。 */
const providers = new ProviderRegistry();
providers.register(createBangumiProvider());

const log = (label: string, value: unknown): void => {
  console.log(`\n===== ${label} =====`);
  console.log(
    typeof value === "string" ? value : JSON.stringify(value, null, 2),
  );
};

export async function runBangumiProbe(
  id: string,
  keyword: string | undefined,
): Promise<void> {
  if (id === undefined || id === "") {
    throw new Error("用法：--probe-bangumi <subjectId> [keyword]");
  }

  const started = Date.now();
  const meta = await fetchBangumi(id, "tv");
  log("subject", {
    title: meta.title,
    originalTitle: meta.originalTitle,
    premiered: meta.premiered,
    studios: meta.studios,
    ratings: meta.ratings,
    uniqueIds: meta.uniqueIds,
    poster: meta.posterUrl,
    seasonCount: meta.seasonCount,
    episodeCount: meta.episodeCount,
    plot: meta.plot?.slice(0, 80),
  });
  log("fetch ms (miss)", Date.now() - started);

  const cachedStart = Date.now();
  await fetchBangumi(id, "tv");
  log("fetch ms (cache)", Date.now() - cachedStart);

  const results = await searchBangumi(keyword ?? meta.title);
  log("search top3", results.slice(0, 3));

  const episodes = await fetchBangumiEpisodes(id);
  log("episodes", {
    count: episodes.length,
    sample: episodes.slice(0, 3).map((e) => ({
      episode: e.episode,
      title: e.title,
      aired: e.aired,
      runtime: e.runtimeMinutes,
    })),
  });

  const preview = await scrapePreview(providers, "bangumi", id, "tv");
  log("preview", {
    title: preview.metadata.title,
    seasons: preview.metadata.seasonCount,
    episodes: preview.episodes.length,
  });
}

/** 写盘探针：只对可丢弃目录使用 */
export async function runBangumiApplyProbe(
  dir: string,
  id: string,
): Promise<void> {
  if (dir === undefined || dir === "" || id === undefined || id === "") {
    throw new Error("用法：--probe-bangumi-apply <可丢弃目录> <subjectId>");
  }
  const result = await scrapeApply(providers, dir, "bangumi", id, "tv");
  log("apply", result);
  log("files", await readdir(dir));
}
