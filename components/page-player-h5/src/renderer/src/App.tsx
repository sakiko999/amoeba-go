/**
 * player-h5 页面：宿主打开/复用本 tab 时带上播放目标（namespace + streams），
 * 交给移植的 PlayableMedia 播放。列在这里的流都是原生播放器放不了的
 * （DASH/FLV/需要自定义请求头），媒体请求经宿主 fetch 插件发出。
 */

import { useEffect, useState } from "react";

import { onPlay, type PlayTarget, target } from "./host";
import { PlayableMedia } from "./player/index.ts";

export function App() {
  const [current, setCurrent] = useState<PlayTarget | null>(null);

  useEffect(() => {
    let alive = true;
    void target()
      .then((t) => {
        if (alive && t) setCurrent(t);
      })
      .catch((err: unknown) => console.warn("player-h5: 取播放目标失败", err));
    onPlay((t) => setCurrent(t));
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className="h-full w-full bg-black">
      {current ? (
        /* 舞台自带视口内自封顶(MediaState 固定 16:9;VideoShell 按视频真实比例出黑边),
           这里只负责居中 —— 黑边一律落在页面黑底上。 */
        <div className="flex h-full w-full items-stretch justify-stretch">
          <PlayableMedia
            key={current.url}
            streams={current.streams}
            live={current.kind === "live"}
            showDanmaku={false}
            onError={(err) => console.warn("player-h5: 播放失败", err)}
          />
        </div>
      ) : (
        <div className="flex h-full flex-col items-center justify-center gap-2 px-6 text-center">
          <p className="text-sm font-medium text-neutral-200">从搜索结果播放</p>
          <p className="max-w-[44ch] text-xs leading-relaxed text-neutral-500">
            在「搜索」页点击结果卡片：原生播放器放不了的流（DASH / FLV / 需要防盗链请求头）会在这里由
            hls.js / dash.js 播放。
          </p>
        </div>
      )}
    </div>
  );
}
