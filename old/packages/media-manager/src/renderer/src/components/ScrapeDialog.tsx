/**
 * 刮削面板：来源 → 搜索/输入 ID → 预览 → 写入作品 NFO + 每集 NFO + 海报。
 * Bangumi 免 token 为默认来源；TMDB 需 TMDB_TOKEN；豆瓣无搜索、只支持按 ID。
 * 写入目标固定为当前作品目录（只由用户在此显式触发）。
 */

import { useCallback, useEffect, useState } from "react";
import { Search, Sparkles } from "lucide-react";
import {
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  ScrollArea,
} from "@demo/ui";
import type {
  LibraryItem,
  ScrapePreview,
  ScrapeSource,
  SearchResult,
} from "@tabs/media-library";
import { media, usePoster } from "../lib/api";

const SOURCES: { value: ScrapeSource; label: string }[] = [
  { value: "bangumi", label: "Bangumi" },
  { value: "tmdb", label: "TMDB" },
  { value: "douban", label: "豆瓣" },
];

const errorText = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

interface Props {
  item: LibraryItem;
  open: boolean;
  onOpenChange(open: boolean): void;
  onApplied(): void;
}

export function ScrapeDialog({
  item,
  open,
  onOpenChange,
  onApplied,
}: Props): React.JSX.Element {
  const [source, setSource] = useState<ScrapeSource>("bangumi");
  const [keyword, setKeyword] = useState(item.title);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [selected, setSelected] = useState<SearchResult | null>(null);
  const [preview, setPreview] = useState<ScrapePreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [applied, setApplied] = useState<string | null>(null);
  const poster = usePoster(item.posterLocal);

  const runSearch = useCallback(
    async (src: ScrapeSource, kw: string): Promise<void> => {
      if (kw.trim() === "") return;
      setBusy(true);
      setError(null);
      setSelected(null);
      setPreview(null);
      setApplied(null);
      try {
        setResults(
          await media().searchMetadata(src, kw.trim(), item.mediaType),
        );
      } catch (err) {
        setResults([]);
        setError(errorText(err));
      } finally {
        setBusy(false);
      }
    },
    [item.mediaType],
  );

  const pick = useCallback(
    async (src: ScrapeSource, hit: SearchResult): Promise<void> => {
      setSelected(hit);
      setPreview(null);
      setApplied(null);
      setError(null);
      setBusy(true);
      try {
        setPreview(await media().scrapePreview(src, hit.id, item.mediaType));
      } catch (err) {
        setError(errorText(err));
      } finally {
        setBusy(false);
      }
    },
    [item.mediaType],
  );

  useEffect(() => {
    if (!open) return;
    setSource("bangumi");
    setKeyword(item.title);
    setResults([]);
    setSelected(null);
    setPreview(null);
    setError(null);
    setApplied(null);
    void runSearch("bangumi", item.title);
  }, [open, item.title, runSearch]);

  const switchSource = (next: ScrapeSource): void => {
    setSource(next);
    setResults([]);
    setSelected(null);
    setPreview(null);
    setError(null);
    setApplied(null);
  };

  const apply = async (): Promise<void> => {
    if (selected === null) return;
    setBusy(true);
    setError(null);
    try {
      const result = await media().scrapeApply(
        item.dir,
        source,
        selected.id,
        item.mediaType,
      );
      const extras = [
        result.episodeNfos > 0 ? `${result.episodeNfos} 个单集 NFO` : "",
        result.posterPath !== undefined ? "海报" : "",
      ].filter(Boolean);
      setApplied(
        `已写入 ${result.nfoPath}${extras.length > 0 ? `（含 ${extras.join("、")}）` : ""}`,
      );
      onApplied();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const meta = preview?.metadata;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="size-4" />
            刮削《{item.title}》
          </DialogTitle>
          <DialogDescription>
            搜索并预览元数据，写入 tvshow.nfo / movie.nfo、每集 NFO
            与海报。已有文件会被覆盖。
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-wrap gap-1.5">
          {SOURCES.map((s) => (
            <Button
              key={s.value}
              size="sm"
              variant={source === s.value ? "default" : "outline"}
              disabled={busy}
              onClick={() => switchSource(s.value)}
            >
              {s.label}
            </Button>
          ))}
          {source === "douban" && (
            <span className="self-center text-xs text-muted-foreground">
              豆瓣无搜索，输入 subject ID 后点「预览」
            </span>
          )}
        </div>

        <div className="flex gap-2">
          <Input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== "Enter" || busy) return;
              if (source === "douban") {
                void pick("douban", {
                  source: "douban",
                  id: keyword.trim(),
                  title: keyword.trim(),
                  mediaType: item.mediaType,
                });
              } else {
                void runSearch(source, keyword);
              }
            }}
            placeholder={source === "douban" ? "豆瓣 subject ID" : "搜索关键词"}
          />
          <Button
            variant="secondary"
            className="gap-1.5"
            disabled={busy || keyword.trim() === ""}
            onClick={() => {
              if (source === "douban") {
                void pick("douban", {
                  source: "douban",
                  id: keyword.trim(),
                  title: keyword.trim(),
                  mediaType: item.mediaType,
                });
              } else {
                void runSearch(source, keyword);
              }
            }}
          >
            <Search className="size-4" />
            {source === "douban" ? "预览" : "搜索"}
          </Button>
        </div>

        <div className="grid min-h-72 grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_15rem]">
          <div className="flex min-h-0 flex-col rounded-lg border">
            <div className="border-b px-3 py-1.5 text-xs text-muted-foreground">
              {busy ? "查询中…" : `${results.length} 条结果`}
            </div>
            <ScrollArea className="h-64">
              <ul className="divide-y">
                {results.map((hit) => (
                  <li key={`${hit.source}:${hit.id}`}>
                    <button
                      type="button"
                      className={`w-full px-3 py-2 text-left text-sm hover:bg-accent ${
                        selected?.id === hit.id ? "bg-accent" : ""
                      }`}
                      onClick={() => void pick(source, hit)}
                    >
                      <div className="truncate font-medium">{hit.title}</div>
                      <div className="truncate text-xs text-muted-foreground">
                        {[hit.year, hit.originalTitle, `#${hit.id}`]
                          .filter(Boolean)
                          .join(" · ")}
                      </div>
                    </button>
                  </li>
                ))}
                {results.length === 0 && (
                  <li className="px-3 py-6 text-center text-xs text-muted-foreground">
                    {source === "douban"
                      ? "输入 ID 后预览"
                      : "没有结果，换个关键词试试"}
                  </li>
                )}
              </ul>
            </ScrollArea>
          </div>

          <div className="min-w-0 overflow-hidden rounded-lg border">
            {poster && (
              <img
                src={poster}
                alt=""
                className="aspect-[2/3] w-full object-cover"
              />
            )}
            <div className="space-y-1.5 p-3 text-xs">
              <div className="text-sm font-medium">
                {meta?.title ?? "未选择条目"}
              </div>
              {meta?.originalTitle !== undefined && (
                <div className="text-muted-foreground">
                  {meta.originalTitle}
                </div>
              )}
              <div className="flex flex-wrap gap-1">
                {meta?.premiered !== undefined && (
                  <Badge variant="secondary">{meta.premiered}</Badge>
                )}
                {meta?.ratings[0] !== undefined && (
                  <Badge variant="secondary">
                    {meta.ratings[0].value.toFixed(1)} 分
                  </Badge>
                )}
                {meta?.episodeCount !== undefined && (
                  <Badge variant="secondary">{meta.episodeCount} 集</Badge>
                )}
              </div>
              {meta?.studios.map((s) => (
                <div key={s} className="truncate text-muted-foreground">
                  {s}
                </div>
              ))}
              {preview !== null && (
                <div className="text-muted-foreground">
                  {preview.episodes.length > 0
                    ? `将写入 ${preview.episodes.length} 个单集 NFO`
                    : "无单集数据"}
                </div>
              )}
            </div>
          </div>
        </div>

        {meta?.plot !== undefined && (
          <ScrollArea className="h-24 rounded-lg border p-3 text-xs leading-relaxed whitespace-pre-line text-muted-foreground">
            {meta.plot}
          </ScrollArea>
        )}

        {error !== null && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {error}
          </p>
        )}
        {applied !== null && (
          <p className="rounded-md border border-primary/40 bg-primary/10 px-3 py-2 text-xs">
            {applied}
          </p>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
          <Button
            className="gap-1.5"
            disabled={busy || selected === null || preview === null}
            onClick={() => void apply()}
          >
            <Sparkles className="size-4" />
            写入 NFO 与海报
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
