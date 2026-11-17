/**
 * flv.js 的最小类型声明（上游包未带类型）。只声明播放器实际用到的面。
 */

declare module "flv.js" {
  namespace flvjs {
    interface MediaDataSource {
      type: "flv"
      url: string
      isLive?: boolean
    }

    interface Player {
      attachMediaElement(el: HTMLMediaElement): void
      load(): void
      play(): Promise<void>
      destroy(): void
      on(event: string, listener: (...args: unknown[]) => void): void
    }
  }

  const flvjs: {
    isSupported(): boolean
    createPlayer(
      media: flvjs.MediaDataSource,
      config?: Record<string, unknown>,
    ): flvjs.Player
    Events: { ERROR: string }
  }

  export default flvjs
}
