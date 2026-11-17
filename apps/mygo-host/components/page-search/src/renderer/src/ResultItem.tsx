/**
 * 单条结果：缩略图（无图/加载失败回退平台色块）+ 标题 + 次行（主播/人气/房号）
 * + 直播状态徽标。整卡点击 = 宿主原生播放器播放（Player.Play 经网关解析平台流），
 * 失败在卡内显示错误；外链打开是后续项（播放为主诉求）。
 */

import { useState } from "react";
import { Badge } from "@demo/ui";
import { play, type Item } from "./api";
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
  const canPlay = item.url !== undefined && item.url !== "";
  const onPlay = (): void => {
    if (!canPlay) return;
    setPlayError(null);
    play(item, namespace).catch((e: unknown) => {
      setPlayError(e instanceof Error ? e.message : String(e));
    });
  };
  const color = platformColor(namespace);
  const name = platformName(namespace, label);
  const meta: string[] = [];
  if (item.author?.name !== undefined && item.author.name !== "") {
    meta.push(item.author.name);
  }
  if (item.online !== undefined) meta.push(`${formatCount(item.online)} 人气`);
  if (item.roomId !== undefined && item.roomId !== "") {
    meta.push(`房号 ${item.roomId}`);
  }
  const image =
    item.thumbnail !== undefined && item.thumbnail !== ""
      ? item.thumbnail
      : item.author?.avatar;
  const content = (
    <>
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
        {meta.length > 0 && (
          <p className="truncate text-xs text-muted-foreground">
            {meta.join(" · ")}
          </p>
        )}
        {item.summary !== undefined && item.summary !== "" && (
          <p className="line-clamp-1 text-xs text-muted-foreground/80">
            {item.summary}
          </p>
        )}
        {playError !== null && (
          <p className="line-clamp-1 text-xs text-red-500">{playError}</p>
        )}
        <div className="mt-auto flex items-center gap-1.5 text-xs text-muted-foreground">
          <span
            className="size-2 shrink-0 rounded-full"
            style={{ backgroundColor: color }}
          />
          {name}
        </div>
      </div>
    </>
  );
  const cls =
    "flex w-full gap-3 rounded-xl border bg-card p-3 text-left transition-colors focus-visible:outline-2 focus-visible:outline-primary" +
    (canPlay ? " cursor-pointer hover:border-primary/50" : "");
  return (
    <button type="button" onClick={onPlay} disabled={!canPlay} className={cls}>
      {content}
    </button>
  );
}
