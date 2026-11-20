/**
 * 播放器包的本地类型面：对应上游 @amoeba/core 的 MediaStream/PlaybackChapter/
 * ResolvePlayback 与弹幕 DTO 的播放器本地形态,不再依赖旧 amoeba 包图。
 * MediaStream 与 Go 侧 platform.Stream 的 JSON 同形。
 */

export interface MediaStream {
  url: string
  format?: string
  headers?: Record<string, string>
  /** 档位名（直播多清晰度流，如 douyu 的「原画2K60/蓝光8M」；单流无）。 */
  quality?: string
  /** 档位原始值（直播平台切档参数，如 douyu 的 rate）。 */
  rate?: number
  /** DASH manifest（MPD XML）——音视频分离流（bili/youtube）用 dash.js 合成播放。 */
  dashManifest?: string
}

/** 章节时间点（秒 + 标题）。 */
export interface PlaybackChapter {
  t: number
  label: string
}

export interface DanmakuItem {
  text: string
  user?: string
  color?: string
  timeMs?: number
  mode?: number
}

/** 弹幕流：订阅即开始，onItems 收批次；返回 unsubscribe。 */
export type DanmakuStream = (onItems: (items: DanmakuItem[]) => void) => () => void

/** 懒解析结果（PlayableMedia.resolve 的返回；mygo 宿主直接经 streams 转入）。 */
export interface ResolvePlayback {
  streams: MediaStream[]
  chapters?: PlaybackChapter[]
  danmaku?: DanmakuStream
}
