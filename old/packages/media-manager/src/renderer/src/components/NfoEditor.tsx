/**
 * NFO 编辑：读取作品 NFO（没有则用目录名/扫描结果初始化）→ 表单 → 写回。
 * 演员与 uniqueid 不在此编辑，但保存时原样保留，避免丢数据。
 * 评分只编辑第一个 rating（保留其 source），没有评分时用 "manual"。
 */

import { useEffect, useState } from "react";
import { FilePenLine } from "lucide-react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Textarea,
} from "@demo/ui";
import type { LibraryItem, MediaMetadata } from "@tabs/media-library";
import { media } from "../lib/api";

const errorText = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

interface Form {
  title: string;
  originalTitle: string;
  premiered: string;
  status: string;
  genres: string;
  studios: string;
  rating: string;
  votes: string;
  plot: string;
}

const emptyForm: Form = {
  title: "",
  originalTitle: "",
  premiered: "",
  status: "",
  genres: "",
  studios: "",
  rating: "",
  votes: "",
  plot: "",
};

const splitList = (s: string): string[] =>
  s
    .split(/[,，、]/)
    .map((v) => v.trim())
    .filter(Boolean);

function toForm(meta: MediaMetadata): Form {
  const rating = meta.ratings[0];
  return {
    title: meta.title,
    originalTitle: meta.originalTitle ?? "",
    premiered: meta.premiered ?? "",
    status: meta.status ?? "",
    genres: meta.genres.join("，"),
    studios: meta.studios.join("，"),
    rating: rating !== undefined ? String(rating.value) : "",
    votes: rating?.votes !== undefined ? String(rating.votes) : "",
    plot: meta.plot ?? "",
  };
}

interface Props {
  item: LibraryItem;
  open: boolean;
  onOpenChange(open: boolean): void;
  onSaved(): void;
}

export function NfoEditor({
  item,
  open,
  onOpenChange,
  onSaved,
}: Props): React.JSX.Element {
  const [form, setForm] = useState<Form>(emptyForm);
  const [original, setOriginal] = useState<MediaMetadata | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setError(null);
    let alive = true;
    void media()
      .readNfo(item.dir)
      .then((meta) => {
        if (!alive) return;
        setOriginal(meta);
        setForm(
          meta !== null
            ? toForm(meta)
            : toForm({
                mediaType: item.mediaType,
                title: item.title,
                originalTitle: item.originalTitle,
                plot: item.plot,
                premiered: item.year,
                genres: item.genres,
                studios: [],
                ratings: [],
                actors: [],
                uniqueIds: [],
              }),
        );
      })
      .catch((err: unknown) => {
        if (alive) setError(errorText(err));
      });
    return () => {
      alive = false;
    };
  }, [open, item]);

  const set = (key: keyof Form, value: string): void => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const save = async (): Promise<void> => {
    if (form.title.trim() === "") {
      setError("标题不能为空");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const base: MediaMetadata = original ?? {
        mediaType: item.mediaType,
        title: item.title,
        genres: [],
        studios: [],
        ratings: [],
        actors: [],
        uniqueIds: [],
      };
      const ratings =
        form.rating.trim() === ""
          ? []
          : [
              {
                source: base.ratings[0]?.source ?? "manual",
                value: Number(form.rating),
                votes:
                  form.votes.trim() === "" ? undefined : Number(form.votes),
              },
            ];
      await media().writeNfo(item.dir, {
        ...base,
        title: form.title.trim(),
        originalTitle: form.originalTitle.trim() || undefined,
        premiered: form.premiered.trim() || undefined,
        status: form.status.trim() || undefined,
        genres: splitList(form.genres),
        studios: splitList(form.studios),
        ratings,
        plot: form.plot.trim() || undefined,
      });
      onSaved();
      onOpenChange(false);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FilePenLine className="size-4" />
            编辑 NFO
          </DialogTitle>
          <DialogDescription>
            写回 {item.mediaType === "tv" ? "tvshow.nfo" : "movie.nfo"}
            ；演员与外部 ID 保持原样。
          </DialogDescription>
        </DialogHeader>

        <div className="grid max-h-[60vh] gap-3 overflow-y-auto pr-1">
          <div className="space-y-1.5">
            <Label htmlFor="nfo-title">标题 *</Label>
            <Input
              id="nfo-title"
              value={form.title}
              onChange={(e) => set("title", e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="nfo-original">原名</Label>
            <Input
              id="nfo-original"
              value={form.originalTitle}
              onChange={(e) => set("originalTitle", e.target.value)}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="nfo-premiered">首映日期</Label>
              <Input
                id="nfo-premiered"
                placeholder="yyyy-mm-dd"
                value={form.premiered}
                onChange={(e) => set("premiered", e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="nfo-status">状态</Label>
              <Input
                id="nfo-status"
                placeholder="Ended / Continuing"
                value={form.status}
                onChange={(e) => set("status", e.target.value)}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="nfo-genres">类型（逗号分隔）</Label>
              <Input
                id="nfo-genres"
                value={form.genres}
                onChange={(e) => set("genres", e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="nfo-studios">工作室（逗号分隔）</Label>
              <Input
                id="nfo-studios"
                value={form.studios}
                onChange={(e) => set("studios", e.target.value)}
              />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="nfo-rating">评分（0-10）</Label>
              <Input
                id="nfo-rating"
                type="number"
                min={0}
                max={10}
                step={0.1}
                value={form.rating}
                onChange={(e) => set("rating", e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="nfo-votes">评分人数</Label>
              <Input
                id="nfo-votes"
                type="number"
                min={0}
                value={form.votes}
                onChange={(e) => set("votes", e.target.value)}
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="nfo-plot">简介</Label>
            <Textarea
              id="nfo-plot"
              rows={6}
              value={form.plot}
              onChange={(e) => set("plot", e.target.value)}
            />
          </div>

          {error !== null && (
            <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              {error}
            </p>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button disabled={busy} onClick={() => void save()}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
