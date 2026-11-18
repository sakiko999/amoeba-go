/**
 * 2:3 海报卡：标题条在海报外常驻（扫视可辨认，也不与海报自带文字重叠）。
 * 海报经 IPC data-url 加载（CSP 不放行 file://，见 usePoster）。
 */

import { Star } from "lucide-react";
import type { LibraryItem } from "@tabs/media-library";
import { usePoster } from "../lib/api";

export function PosterCard({
  item,
  onOpen,
}: {
  item: LibraryItem;
  onOpen(): void;
}): React.JSX.Element {
  const poster = usePoster(item.posterLocal);
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group flex flex-col gap-1.5 rounded-lg text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <div className="relative aspect-[2/3] w-full overflow-hidden rounded-lg bg-muted">
        {poster ? (
          <img
            src={poster}
            alt={item.title}
            loading="lazy"
            className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full items-center justify-center p-2 text-center text-xs text-muted-foreground">
            {poster === null ? item.title : "…"}
          </div>
        )}
        {item.rating !== undefined && (
          <span className="absolute top-1.5 right-1.5 flex items-center gap-0.5 rounded bg-black/70 px-1 py-0.5 text-[11px] font-medium text-primary">
            <Star className="size-3 fill-current" />
            {item.rating.toFixed(1)}
          </span>
        )}
        {!item.hasNfo && (
          <span className="absolute top-1.5 left-1.5 rounded bg-black/70 px-1 py-0.5 text-[10px] text-muted-foreground">
            未刮削
          </span>
        )}
      </div>
      <div className="min-w-0">
        <div className="truncate text-xs font-medium">{item.title}</div>
        <div className="text-[11px] text-muted-foreground">
          {item.year ?? "—"} · {item.mediaType === "tv" ? "剧集" : "电影"} ·{" "}
          {item.videoCount} 集
        </div>
      </div>
    </button>
  );
}
