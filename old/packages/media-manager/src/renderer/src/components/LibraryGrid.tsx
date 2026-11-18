/**
 * 海报墙：genre 筛选 chips（由库数据派生）+ 骨架/空态/错误态。
 */

import { FolderOpen } from "lucide-react";
import type { LibraryItem } from "@tabs/media-library";
import { Button, Skeleton } from "@demo/ui";
import { PosterCard } from "./PosterCard";

interface LibraryGridProps {
  items: LibraryItem[];
  genres: string[];
  genre: string | null;
  onGenre(genre: string | null): void;
  loading: boolean;
  error: string | null;
  hasRoot: boolean;
  onPick(): void;
  onOpen(item: LibraryItem): void;
}

export function LibraryGrid({
  items,
  genres,
  genre,
  onGenre,
  loading,
  error,
  hasRoot,
  onPick,
  onOpen,
}: LibraryGridProps): React.JSX.Element {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {genres.length > 0 && (
        <div className="flex flex-wrap gap-1.5 px-4 pt-3 pb-1">
          <Chip active={genre === null} onClick={() => onGenre(null)}>
            全部
          </Chip>
          {genres.map((g) => (
            <Chip key={g} active={genre === g} onClick={() => onGenre(g)}>
              {g}
            </Chip>
          ))}
        </div>
      )}

      <div className="grid min-h-0 flex-1 auto-rows-min gap-4 overflow-y-auto px-4 pt-2 pb-6 [grid-template-columns:repeat(auto-fill,minmax(132px,1fr))]">
        {loading
          ? Array.from({ length: 12 }, (_, i) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: 纯展示的静态骨架，顺序固定
              <div key={i} className="flex flex-col gap-1.5">
                <Skeleton className="aspect-[2/3] w-full rounded-lg" />
                <Skeleton className="h-3.5 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            ))
          : items.map((item) => (
              <PosterCard
                key={item.id}
                item={item}
                onOpen={() => onOpen(item)}
              />
            ))}
      </div>

      {!loading && (error !== null || !hasRoot || items.length === 0) && (
        <div className="pointer-events-none fixed inset-0 top-11 flex items-center justify-center">
          <div className="pointer-events-auto flex max-w-sm flex-col items-center gap-3 text-center">
            {error !== null ? (
              <>
                <p className="text-sm text-muted-foreground">
                  扫描失败：{error}
                </p>
                <Button variant="outline" size="sm" onClick={onPick}>
                  重新选择目录
                </Button>
              </>
            ) : !hasRoot ? (
              <>
                <FolderOpen className="size-8 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">
                  先选择一个媒体库目录
                </p>
                <Button size="sm" onClick={onPick}>
                  选择媒体库
                </Button>
              </>
            ) : (
              <p className="text-sm text-muted-foreground">
                这个目录里没有认出任何作品
              </p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick(): void;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full border px-2.5 py-1 text-xs transition-colors ${
        active
          ? "border-transparent bg-primary text-primary-foreground"
          : "bg-transparent text-muted-foreground hover:bg-accent hover:text-accent-foreground"
      }`}
    >
      {children}
    </button>
  );
}
