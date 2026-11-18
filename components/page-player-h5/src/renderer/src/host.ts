/**
 * player-h5 与宿主的契约（Go 侧 components/player/service.go 的 H5Target
 * 与 tabs.Manager.OpenPlayerH5）。首次播放目标在打开 tab 时由宿主存下，
 * 页面挂载后主动来取；同 tab 复用时的后续目标经 "player.h5.play" 事件推送。
 */

import type { MediaStream } from "./player/types";

/** 宿主播放目标（字段与 Go H5Target 对齐）。 */
export interface PlayTarget {
  instanceId: string;
  title?: string;
  /** live | video 等条目 kind；缺省由播放器按元数据判定。 */
  kind?: string;
  namespace: string;
  url: string;
  streams: MediaStream[];
}

function bridge(): NonNullable<Window["mygo"]> {
  if (window.mygo === undefined) {
    throw new Error("window.mygo 未注入（页面不是 trusted page？）");
  }
  return window.mygo;
}

function instanceId(): string {
  const id = window.tab?.instanceId;
  if (id === undefined) {
    throw new Error("window.tab 未注入（pageBridge 缺失）");
  }
  return id;
}

/** 本 tab 的播放目标；没有（从新建菜单打开）时返回 null。 */
export async function target(): Promise<PlayTarget | null> {
  const raw = await bridge().call("Player.Target", instanceId());
  return (raw as PlayTarget | null) ?? null;
}

/** 监听宿主推来的新目标（同 tab 连续点播）；事件广播到全窗口，按 instanceId 过滤。 */
export function onPlay(cb: (target: PlayTarget) => void): void {
  const id = instanceId();
  bridge().on("player.h5.play", (payload) => {
    const t = payload as PlayTarget;
    if (t.instanceId === id) cb(t);
  });
}
