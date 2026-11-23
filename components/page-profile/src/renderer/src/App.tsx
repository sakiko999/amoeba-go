/**
 * 个人主页：搜索结果点作者进入（宿主复用单个 profile tab，换目标经事件推送）。
 * 数据面：Profile.Target 给进入现场快照（资料 + 直播间），Profile.Videos 按
 * 平台映射到 <ns>.fetch 渠道分页取视频/回看；播放统一经 Player.Play。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, Button } from "@demo/ui";
import { Loader2, Play } from "lucide-react";
import { PlatformIcon } from "platform-icons";
import {
  onOpen,
  play,
  target,
  videos,
  type ProfileTarget,
  type ProfileVideo,
} from "./host";
import { platformColor, platformName } from "./platforms";

export function App(): React.ReactElement {
  const [current, setCurrent] = useState<ProfileTarget | null>(null);

  useEffect(() => {
    let alive = true;
    void target()
      .then((t) => {
        if (alive && t) setCurrent(t);
      })
      .catch((err: unknown) => console.warn("profile: 取主页目标失败", err));
    onOpen((t) => setCurrent(t));
    return () => {
      alive = false;
    };
  }, []);

  if (current === null) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-2 px-6 text-center">
        <p className="text-sm font-medium text-foreground">从搜索结果进入</p>
        <p className="max-w-[44ch] text-xs leading-relaxed text-muted-foreground">
          在「搜索」页点击主播的结果卡片（或任意结果的作者名），这里会展示
          主播/UP 主的资料、直播间与视频/回看列表。
        </p>
      </div>
    );
  }

  return (
    <ProfileView
      key={`${current.instanceId}:${current.platform}:${current.roomId}:${current.uid}`}
      target={current}
    />
  );
}

function ProfileView({ target }: { target: ProfileTarget }): React.ReactElement {
  const [actionError, setActionError] = useState<string | null>(null);
  const onPlay = useCallback(
    (url: string | undefined, title: string, kind: "live" | "video") => {
      if (url === undefined || url === "") return;
      setActionError(null);
      play({ namespace: target.platform, url, title, kind }).catch((e: unknown) => {
        setActionError(e instanceof Error ? e.message : String(e));
      });
    },
    [target.platform],
  );

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 p-6">
      <Header target={target} />
      <LiveCard target={target} onPlay={onPlay} />
      <VideoSection onPlay={onPlay} />
      {actionError !== null && (
        <p className="text-xs text-red-500">播放失败：{actionError}</p>
      )}
    </div>
  );
}

function Header({ target }: { target: ProfileTarget }): React.ReactElement {
  const color = platformColor(target.platform);
  const meta: string[] = [platformName(target.platform)];
  if (target.roomId !== undefined && target.roomId !== "") {
    meta.push(`房号 ${target.roomId}`);
  } else if (target.uid !== undefined && target.uid !== "") {
    meta.push(`ID ${target.uid}`);
  }
  return (
    <header className="flex items-center gap-4">
      <div
        className="flex size-16 shrink-0 items-center justify-center overflow-hidden rounded-full text-2xl font-bold text-white/90"
        style={{ backgroundColor: color }}
      >
        {target.avatar !== undefined && target.avatar !== "" ? (
          <img
            src={target.avatar}
            alt=""
            referrerPolicy="no-referrer"
            className="size-full object-cover"
          />
        ) : (
          target.name.slice(0, 1)
        )}
      </div>
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex items-center gap-2">
          <h1 className="truncate text-lg font-semibold">{target.name}</h1>
          <LiveBadge status={target.liveStatus} />
        </div>
        <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <PlatformIcon namespace={target.platform} />
          {meta.join(" · ")}
        </p>
      </div>
    </header>
  );
}

function LiveBadge({ status }: { status?: string }): React.ReactElement | null {
  if (status !== "live" && status !== "offline") return null;
  return status === "live" ? (
    <Badge className="gap-1 bg-red-500/15 text-red-500 hover:bg-red-500/15">
      <span className="size-1.5 animate-pulse rounded-full bg-current" />
      直播中
    </Badge>
  ) : (
    <Badge variant="outline" className="text-muted-foreground">
      未开播
    </Badge>
  );
}

function LiveCard({
  target,
  onPlay,
}: {
  target: ProfileTarget;
  onPlay: (url: string | undefined, title: string, kind: "live" | "video") => void;
}): React.ReactElement | null {
  if (target.liveTitle === undefined && target.url === undefined) return null;
  const canPlay = target.url !== undefined && target.url !== "";
  const live = target.liveStatus === "live";
  return (
    <section className="flex gap-3 rounded-xl border bg-card p-3">
      <div className="relative aspect-video w-44 shrink-0 overflow-hidden rounded-lg bg-accent">
        {target.liveCover !== undefined && target.liveCover !== "" ? (
          <img
            src={target.liveCover}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            className="size-full object-cover"
          />
        ) : (
          <div
            className="flex size-full items-center justify-center text-lg font-bold text-white/90"
            style={{ backgroundColor: platformColor(target.platform) }}
          >
            {target.name.slice(0, 1)}
          </div>
        )}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <p className="text-xs text-muted-foreground">直播间</p>
        <p className="line-clamp-2 text-sm font-medium">
          {target.liveTitle !== undefined && target.liveTitle !== ""
            ? target.liveTitle
            : target.name}
        </p>
        {target.liveOnline !== undefined && target.liveOnline > 0 && (
          <p className="text-xs text-muted-foreground">
            {formatCount(target.liveOnline)} 人气
          </p>
        )}
        <Button
          size="sm"
          className="mt-auto w-fit gap-1"
          disabled={!canPlay}
          onClick={() => onPlay(target.url, target.liveTitle ?? target.name, "live")}
        >
          <Play className="size-3.5" />
          {live || target.liveStatus === "" ? "进入直播间" : "进入直播间（未开播）"}
        </Button>
      </div>
    </section>
  );
}

function VideoSection({
  onPlay,
}: {
  onPlay: (url: string | undefined, title: string, kind: "live" | "video") => void;
}): React.ReactElement {
  const [items, setItems] = useState<ProfileVideo[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState<"first" | "more" | null>("first");
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);

  const load = useCallback(async (next: string | undefined) => {
    const gen = ++generation.current;
    setLoading(next === undefined ? "first" : "more");
    setError(null);
    try {
      const res = await videos(next);
      if (gen !== generation.current) return;
      setItems((prev) => (next === undefined ? res.items : [...prev, ...res.items]));
      setCursor(res.cursor);
      setHasMore(res.hasMore === true || (res.cursor !== undefined && res.cursor !== ""));
    } catch (err: unknown) {
      if (gen !== generation.current) return;
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (gen === generation.current) setLoading(null);
    }
  }, []);

  useEffect(() => {
    void load(undefined);
    return () => {
      generation.current += 1;
    };
  }, [load]);

  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-semibold">视频 / 回看</h2>
        {loading === "first" && (
          <Loader2 className="size-3.5 animate-spin text-muted-foreground" />
        )}
      </div>
      {error !== null && <p className="text-xs text-red-500">{error}</p>}
      {error === null && loading === "first" && items.length === 0 && (
        <p className="text-xs text-muted-foreground">加载中…</p>
      )}
      {error === null && loading !== "first" && items.length === 0 && (
        <p className="text-xs text-muted-foreground">暂无视频/回看</p>
      )}
      <div className="flex flex-col gap-2">
        {items.map((video) => (
          <VideoRow key={video.id} video={video} onPlay={onPlay} />
        ))}
      </div>
      {hasMore && (
        <Button
          variant="outline"
          size="sm"
          className="w-fit"
          disabled={loading !== null}
          onClick={() => void load(cursor)}
        >
          {loading === "more" ? (
            <Loader2 className="size-3.5 animate-spin" />
          ) : (
            "加载更多"
          )}
        </Button>
      )}
    </section>
  );
}

function VideoRow({
  video,
  onPlay,
}: {
  video: ProfileVideo;
  onPlay: (url: string | undefined, title: string, kind: "live" | "video") => void;
}): React.ReactElement {
  const canPlay = video.url !== undefined && video.url !== "";
  return (
    <button
      type="button"
      disabled={!canPlay}
      onClick={() => onPlay(video.url, video.title, "video")}
      className="flex w-full gap-3 rounded-xl border bg-card p-2 text-left transition-colors enabled:hover:border-primary/50 disabled:opacity-70"
    >
      <div className="aspect-video w-36 shrink-0 overflow-hidden rounded-lg bg-accent">
        {video.thumbnail !== undefined && video.thumbnail !== "" ? (
          <img
            src={video.thumbnail}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            className="size-full object-cover"
          />
        ) : null}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1 py-0.5">
        <span className="line-clamp-2 text-sm font-medium">{video.title}</span>
        {video.summary !== undefined && video.summary !== "" && (
          <span className="line-clamp-1 text-xs text-muted-foreground">
            {video.summary}
          </span>
        )}
      </div>
    </button>
  );
}

function formatCount(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1).replace(/\.0$/, "")}万`;
  return String(n);
}
