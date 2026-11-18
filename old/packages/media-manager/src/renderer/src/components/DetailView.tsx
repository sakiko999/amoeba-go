/**
 * 详情页：海报 + NFO 元信息 + 季/集列表（每集读回同名 NFO 的标题/日期/时长）。
 * 动作：在资源管理器中显示（只读）、刮削、编辑 NFO、整理（写操作，见各自对话框）。
 */

import { useState } from "react";
import {
  ArrowLeft,
  FilePenLine,
  FolderInput,
  FolderSearch,
  Sparkles,
  Star,
} from "lucide-react";
import type { ShowDetail } from "@tabs/media-library";
import { Button } from "@demo/ui";
import { media, usePoster } from "../lib/api";
import { NfoEditor } from "./NfoEditor";
import { OrganizeDialog } from "./OrganizeDialog";
import { ScrapeDialog } from "./ScrapeDialog";

function formatSize(bytes: number | undefined): string {
  if (bytes === undefined) return "—";
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  return `${(bytes / 1024 ** 2).toFixed(0)} MB`;
}

export function DetailView({
  detail,
  root,
  onBack,
  onChanged,
}: {
  detail: ShowDetail;
  root: string | null;
  onBack(): void;
  onChanged(): void;
}): React.JSX.Element {
  const { item, seasons } = detail;
  const poster = usePoster(item.posterLocal);
  const [scrapeOpen, setScrapeOpen] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [organizeOpen, setOrganizeOpen] = useState(false);
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="sticky top-0 z-10 flex items-center gap-2 border-b bg-background/95 px-4 py-2 backdrop-blur">
        <Button variant="ghost" size="sm" className="gap-1.5" onClick={onBack}>
          <ArrowLeft className="size-4" />
          返回
        </Button>
        <span className="truncate text-sm font-medium">{item.title}</span>
      </div>

      <div className="flex flex-wrap gap-6 px-6 pt-5 pb-6">
        <div className="aspect-[2/3] w-40 shrink-0 overflow-hidden rounded-lg bg-muted">
          {poster && (
            <img
              src={poster}
              alt={item.title}
              className="h-full w-full object-cover"
            />
          )}
        </div>
        <div className="min-w-64 max-w-2xl flex-1 space-y-3">
          <h1 className="text-xl font-semibold">
            {item.title}
            {item.year !== undefined && (
              <span className="ml-2 text-base font-normal text-muted-foreground">
                ({item.year})
              </span>
            )}
          </h1>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
            {item.rating !== undefined && (
              <span className="flex items-center gap-1 font-medium text-primary">
                <Star className="size-3.5 fill-current" />
                {item.rating.toFixed(1)}
              </span>
            )}
            <span>{item.mediaType === "tv" ? "剧集" : "电影"}</span>
            {item.seasonCount !== undefined && (
              <span>{item.seasonCount} 季</span>
            )}
            <span>{item.videoCount} 个视频</span>
            {item.genres.map((g) => (
              <span key={g} className="rounded-full border px-2 py-0.5">
                {g}
              </span>
            ))}
          </div>
          {item.plot !== undefined && (
            <p className="text-sm leading-relaxed whitespace-pre-line text-muted-foreground">
              {item.plot}
            </p>
          )}
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              className="gap-1.5"
              onClick={() => setScrapeOpen(true)}
            >
              <Sparkles className="size-4" />
              刮削
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={() => setEditorOpen(true)}
            >
              <FilePenLine className="size-4" />
              编辑 NFO
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              disabled={root === null}
              onClick={() => setOrganizeOpen(true)}
            >
              <FolderInput className="size-4" />
              整理
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={() => media().reveal(item.dir)}
            >
              <FolderSearch className="size-4" />
              在资源管理器中显示
            </Button>
          </div>
        </div>
      </div>

      {seasons.map(({ season, episodes }) => (
        <section key={season} className="px-6 pb-6">
          <h2 className="mb-2 text-sm font-medium">
            {season === 0 ? "特典（Season 00）" : `第 ${season} 季`}
            <span className="ml-2 text-xs font-normal text-muted-foreground">
              {episodes.length} 集
            </span>
          </h2>
          <ul className="divide-y rounded-lg border">
            {episodes.map((ep) => (
              <li
                key={ep.path}
                className="flex items-center gap-3 px-3 py-2 text-sm"
              >
                <span className="w-10 shrink-0 text-center font-medium tabular-nums text-muted-foreground">
                  {ep.episode !== undefined
                    ? `E${String(ep.episode).padStart(2, "0")}`
                    : "—"}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate" title={ep.fileName}>
                    {ep.title ?? ep.fileName}
                  </div>
                  <div className="text-[11px] text-muted-foreground">
                    {[
                      ep.aired,
                      ep.runtimeMinutes !== undefined
                        ? `${ep.runtimeMinutes} 分钟`
                        : undefined,
                      formatSize(ep.sizeBytes),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 w-7 shrink-0 p-0"
                  aria-label="在资源管理器中显示"
                  title={ep.path}
                  onClick={() => media().reveal(ep.path)}
                >
                  <FolderSearch className="size-3.5" />
                </Button>
              </li>
            ))}
          </ul>
        </section>
      ))}

      <ScrapeDialog
        item={item}
        open={scrapeOpen}
        onOpenChange={setScrapeOpen}
        onApplied={onChanged}
      />
      <NfoEditor
        item={item}
        open={editorOpen}
        onOpenChange={setEditorOpen}
        onSaved={onChanged}
      />
      <OrganizeDialog
        root={root}
        item={item}
        open={organizeOpen}
        onOpenChange={setOrganizeOpen}
        onApplied={onChanged}
      />
    </div>
  );
}
