/**
 * 媒体数据模型（NFO 领域补充）。
 *
 * 平台统一契约（MediaType / MediaMetadata / EpisodeMetadata / SearchResult /
 * newMetadata / metadataYear …）在 @tabs/platform-contracts —— 平台插件与
 * media-library 共用同一份定义。本文件只保留 NFO 文件名规则（media-library
 * 的领域逻辑），并 re-export 统一模型保持既有导入面。
 */

export * from "@tabs/platform-contracts";

export function episodeNfoFilename(ep: {
  season: number;
  episode: number;
}): string {
  return `S${String(ep.season).padStart(2, "0")}E${String(ep.episode).padStart(2, "0")}.nfo`;
}

export function nfoFilename(mediaType: "tv" | "movie"): "tvshow.nfo" | "movie.nfo" {
  return mediaType === "tv" ? "tvshow.nfo" : "movie.nfo";
}
