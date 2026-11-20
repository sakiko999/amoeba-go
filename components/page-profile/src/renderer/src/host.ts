/**
 * profile 与宿主的契约（Go 侧 components/service-profile/service.go 的 ProfileTarget /
 * Profile.Videos，播放复用 Player.Play）。首次目标在打开 tab 时由宿主存下，
 * 页面挂载后主动来取；同 tab 复用的后续目标经 "profile.open" 事件推送。
 */

/** 宿主主页目标（字段与 Go ProfileTarget 对齐）。 */
export interface ProfileTarget {
  instanceId: string;
  /** 网关 namespace（bili / douyu / huya / twitch / youtube） */
  platform: string;
  name: string;
  avatar?: string;
  /** 直播间 ID；twitch 为频道 login */
  roomId?: string;
  /** bili uid / youtube channelId */
  uid?: string;
  /** 直播间外链（Player.Play 的 url） */
  url?: string;
  /** 进入时搜索现场快照：直播间卡片直接渲染 */
  liveTitle?: string;
  liveStatus?: "live" | "offline" | "unknown" | "";
  liveOnline?: number;
  liveCover?: string;
}

/** 视频/回看列表条目（platform.Item 子集）。 */
export interface ProfileVideo {
  id: string;
  title: string;
  url?: string;
  summary?: string;
  thumbnail?: string;
  author?: { name: string; avatar?: string };
  liveStatus?: string;
  durationSec?: number;
}

export interface VideosResult {
  items: ProfileVideo[];
  hasMore?: boolean;
  cursor?: string;
  total?: number;
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

/** 本 tab 的主页目标；没有（从新建菜单打开）时返回 null。 */
export async function target(): Promise<ProfileTarget | null> {
  const raw = await bridge().call("Profile.Target", instanceId());
  return (raw as ProfileTarget | null) ?? null;
}

/** 监听宿主推来的新目标（同 tab 换主页）；事件广播到全窗口，按 instanceId 过滤。 */
export function onOpen(cb: (target: ProfileTarget) => void): void {
  const id = instanceId();
  bridge().on("profile.open", (payload) => {
    const t = payload as ProfileTarget;
    if (t.instanceId === id) cb(t);
  });
}

/** 视频/回看分页列表；cursor 为空取首页，后续页用上一页返回的 cursor。 */
export async function videos(cursor?: string): Promise<VideosResult> {
  const raw = await bridge().call("Profile.Videos", {
    instanceId: instanceId(),
    cursor,
  });
  return raw as VideosResult;
}

/** 播放一路直播/视频（宿主解析平台流并交给 player-h5）。 */
export async function play(opts: {
  namespace: string;
  url: string;
  title?: string;
  kind: "live" | "video";
}): Promise<void> {
  await bridge().call("Player.Play", opts);
}
