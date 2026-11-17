/**
 * 移植自 tauri-rss 的 @amoeba/player（媒体播放器 + MediaChrome 式独立控件）。
 *
 * 相对上游的适配：@amoeba/core|resolve 的类型面收敛进 types.ts；
 * hls.js/dash.js 的请求出口由 Rust 隧道换成宿主 fetch 插件（hostFetch.ts，
 * 请求从 Go 发出，Referer/UA 规则见 mediaReferrer.ts）；@amoeba/log 用
 * log/shim.ts 替身。其余组件保持上游结构。
 */
export { PlayableMedia } from "./PlayableMedia.tsx"
export { unlockAudioPlayback } from "./PlayableMedia.tsx"
export { useMediaStream } from "./hooks/useMediaStream.ts"
export {
  useStreamSelection,
  isProgressiveVideo,
  isProgressiveAudio,
  isRtmp,
  isStreamingStream,
} from "./hooks/useStreamSelection.ts"
export { useVideoElement, bindFullscreen } from "./hooks/useVideoElement.ts"
export type { VideoPlayState, VideoOps, FullscreenApi } from "./hooks/useVideoElement.ts"
export { createMediaClock, useMediaTime } from "./hooks/mediaClock.ts"
export type { MediaClock, MediaClockState } from "./hooks/mediaClock.ts"
export { loadPlayerPrefs, savePlayerPrefs, applyPlayerPrefs } from "./prefs.ts"
export type { PlayerPrefs } from "./prefs.ts"
export {
  getPlayerContainer,
  setPlayerContainer,
  usePlayerContainer,
  PlayerContainerProvider,
} from "./ui/PlayerContainer.tsx"
export { VideoShell } from "./VideoShell.tsx"
export { DanmakuLayer } from "./danmaku/DanmakuLayer.tsx"
export { AudioShell } from "./AudioShell.tsx"
export { PlayerControls } from "./PlayerControls.tsx"
export type { MediaStream } from "./types.ts"
export type { DanmakuStream } from "./types.ts"

// ── MediaChrome 式独立控件(可自由组合)────────────────────────
export {
  MediaPlayButton,
  MediaMuteButton,
  MediaVolumeSlider,
  MediaTimeRange,
  MediaRateButton,
  MediaQualityButton,
  MediaDanmakuButton,
  MediaFullscreenButton,
  MediaLiveEdgeButton,
  MediaTimeDisplay,
} from "./controls/index.ts"
export type { DanmakuControl } from "./controls/index.ts"
