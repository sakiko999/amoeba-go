/**
 * 单条结果：缩略图（无图/加载失败回退平台色块）+ 标题 + 次行（主播/人气/房号）
 * + 直播状态徽标。整卡点击：直播结果（主播房间）进个人主页，视频结果直接播放
 * （Player.Play 经网关解析平台流）；点作者名同样进个人主页（Profile.Open）。
 * 失败都在卡内显示。
 */

import { useState } from "react";
import { Badge } from "@demo/ui";
import { openProfile, play, type Item } from "./api";
import { platformColor, platformName } from "./platforms";

function formatCount(n: number): string {
  if (n >= 10000) return `${(n / 10000).toFixed(1).replace(/\.0$/, "")}万`;
  return String(n);
}

function LiveBadge({ item }: { item: Item }): React.ReactElement | null {
  if (item.liveStatus !== "live" && item.liveStatus !== "offline") return null;
  return item.liveStatus === "live" ? (
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

interface ResultItemProps {
  item: Item;
  namespace: string;
  label?: string;
}

export function ResultItem({
  item,
  namespace,
  label,
}: ResultItemProps): React.ReactElement {
  const [imgFailed, setImgFailed] = useState(false);
  const [playError, setPlayError] = useState<string | null>(null);
  const [profileError, setProfileError] = useState<string | null>(null);
  const canPlay = item.url !== undefined && item.url !== "";
  // 直播结果（主播房间）整卡进个人主页（主页里有直播间/回看与进入直播间），
  // 视频结果保持直接播放
  const isLive = item.liveStatus === "live" || item.liveStatus === "offline";
  const onPlay = (): void => {
    if (!canPlay) return;
    setPlayError(null);
    play(item, namespace).catch((e: unknown) => {
      setPlayError(e instanceof Error ? e.message : String(e));
    });
  };
  const onOpenProfile = (): void => {
    setProfileError(null);
    openProfile(item, namespace).catch((e: unknown) => {
      setProfileError(e instanceof Error ? e.message : String(e));
    });
  };
  const onCard = (): void => {
    if (isLive) onOpenProfile();
    else onPlay();
  };
  const color = platformColor(namespace);
  const name = platformName(namespace, label);
  const authorName = item.author?.name ?? "";
  const meta: string[] = [];
  if (item.online !== undefined) meta.push(`${formatCount(item.online)} 人气`);
  if (item.roomId !== undefined && item.roomId !== "") {
    meta.push(`房号 ${item.roomId}`);
  }
  const image =
    item.thumbnail !== undefined && item.thumbnail !== ""
      ? item.thumbnail
      : item.author?.avatar;
  const clickable = isLive || canPlay;
  const cls =
    "flex w-full flex-col gap-2 rounded-xl border bg-card p-3 text-left transition-colors" +
    (clickable ? " hover:border-primary/50" : "");
  // 主操作与「进入主页」是两个并列的真实按钮（作者按钮不能嵌在 <button> 里）；
  // 无 URL 的条目（如频道结果）仍可点（主播进主页）。
  return (
    <div className={cls}>
      <button
        type="button"
        onClick={onCard}
        disabled={!clickable}
        className="flex w-full gap-3 rounded-lg text-left focus-visible:outline-2 focus-visible:outline-primary disabled:cursor-default"
      >
        <div className="relative aspect-video w-40 shrink-0 overflow-hidden rounded-lg bg-accent">
          {image !== undefined && !imgFailed ? (
            <img
              src={image}
              alt=""
              loading="lazy"
              referrerPolicy="no-referrer"
              className="size-full object-cover"
              onError={() => setImgFailed(true)}
            />
          ) : (
            <div
              className="flex size-full items-center justify-center text-xl font-bold text-white/90"
              style={{ backgroundColor: color }}
            >
              {name.slice(0, 1)}
            </div>
          )}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="flex items-start gap-2">
            <span className="line-clamp-2 min-w-0 flex-1 text-sm font-medium">
              {item.title}
            </span>
            <LiveBadge item={item} />
          </div>
          {item.summary !== undefined && item.summary !== "" && (
            <p className="line-clamp-1 text-xs text-muted-foreground/80">
              {item.summary}
            </p>
          )}
          {playError !== null && (
            <p className="line-clamp-1 text-xs text-red-500">{playError}</p>
          )}
        </div>
      </button>
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        {authorName !== "" && (
          <button
            type="button"
            className="max-w-40 truncate underline decoration-dotted underline-offset-2 hover:text-foreground"
            onClick={onOpenProfile}
          >
            {authorName}
          </button>
        )}
        {authorName !== "" && meta.length > 0 && <span>·</span>}
        <span className="truncate">{meta.join(" · ")}</span>
        <span className="ml-auto flex shrink-0 items-center gap-1.5">
          <span
            className="size-2 shrink-0 rounded-full"
            style={{ backgroundColor: color }}
          />
          {name}
        </span>
      </div>
      {profileError !== null && (
        <p className="line-clamp-1 text-xs text-red-500">{profileError}</p>
      )}
    </div>
  );
}
