/**
 * 二进制/时长纯函数（无 React、无副作用）。
 */

/**
 * 播放进度/Seek 可用的时长：直播 = 缓冲边缘（bufferedEnd），VOD = duration。
 * 直播无固定时长（duration=0/Infinity），进度条用已缓冲段；VOD 用总时长。
 */
export function playableDuration(state: {
  live: boolean
  bufferedEnd: number
  duration: number
}): number {
  return state.live ? state.bufferedEnd : state.duration
}
