/**
 * 整理面板：强制两段式 —— 先展示完整计划（移动/跳过/已在位），用户确认后才 apply。
 * 目标已存在的文件一律跳过；符号链接目录由主进程直接拒绝（不要移除该闸门）。
 * apply 永远基于主进程重新生成的计划，前端展示的只是预览。
 */

import { useCallback, useEffect, useState } from "react";
import { FolderInput } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  Badge,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  ScrollArea,
} from "@demo/ui";
import type {
  LibraryItem,
  OrganizePlan,
  PlanItem,
} from "@tabs/media-library";
import { media } from "../lib/api";

const errorText = (err: unknown): string =>
  err instanceof Error ? err.message : String(err);

const baseName = (p: string): string =>
  p.split(/[\\/]/).filter(Boolean).at(-1) ?? p;

const relativeTo = (root: string, p: string): string =>
  p.startsWith(root) ? p.slice(root.length).replace(/^[\\/]/, "") : p;

const STATUS_LABEL: Record<PlanItem["status"], string> = {
  move: "移动",
  skip: "跳过",
  skip_already_in_place: "已在位",
};

interface Props {
  root: string | null;
  item: LibraryItem;
  open: boolean;
  onOpenChange(open: boolean): void;
  onApplied(): void;
}

export function OrganizeDialog({
  root,
  item,
  open,
  onOpenChange,
  onApplied,
}: Props): React.JSX.Element {
  const [plan, setPlan] = useState<OrganizePlan | null>(null);
  const [results, setResults] = useState<PlanItem[] | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (): Promise<void> => {
    if (root === null) return;
    setBusy(true);
    setError(null);
    setResults(null);
    try {
      setPlan(await media().planOrganize(root, item.dir));
    } catch (err) {
      setPlan(null);
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }, [root, item.dir]);

  useEffect(() => {
    if (open) void load();
  }, [open, load]);

  const apply = async (): Promise<void> => {
    if (root === null) return;
    setBusy(true);
    setError(null);
    try {
      const applied = await media().applyOrganize(root, item.dir);
      setResults(applied);
      onApplied();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
      setConfirmOpen(false);
    }
  };

  const moved = results?.filter((r) => r.status === "move").length ?? 0;
  const failed = results?.filter((r) => r.status === "skip").length ?? 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FolderInput className="size-4" />
            整理《{item.title}》
          </DialogTitle>
          <DialogDescription>
            把视频移动到规范目录并写入
            NFO。目标已存在一律跳过，绝不覆盖；确认前不会改动任何文件。
          </DialogDescription>
        </DialogHeader>

        {plan !== null && results === null && (
          <>
            <div className="rounded-md border px-3 py-2 text-xs">
              目标目录：
              <span className="ml-1 font-medium break-all">
                {plan.showRoot}
              </span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              <Badge>{plan.toMove} 个移动</Badge>
              <Badge variant="secondary">{plan.alreadyInPlace} 个已在位</Badge>
              <Badge variant="outline">{plan.skipped} 个跳过</Badge>
            </div>
            <ScrollArea className="h-72 rounded-lg border">
              <ul className="divide-y text-xs">
                {plan.items.map((it) => (
                  <li key={it.from} className="px-3 py-2">
                    <div className="flex items-center gap-2">
                      <Badge
                        variant={it.status === "move" ? "default" : "secondary"}
                        className="shrink-0"
                      >
                        {STATUS_LABEL[it.status]}
                      </Badge>
                      <span className="truncate" title={it.from}>
                        {baseName(it.from)}
                      </span>
                    </div>
                    {it.to !== undefined && it.status === "move" && (
                      <div className="mt-0.5 truncate text-muted-foreground">
                        → {relativeTo(plan.showRoot, it.to)}
                      </div>
                    )}
                    {it.reason !== undefined && (
                      <div className="mt-0.5 text-muted-foreground">
                        {it.reason}
                      </div>
                    )}
                  </li>
                ))}
                {plan.items.length === 0 && (
                  <li className="px-3 py-6 text-center text-muted-foreground">
                    没有可整理的视频文件
                  </li>
                )}
              </ul>
            </ScrollArea>
          </>
        )}

        {results !== null && (
          <div className="rounded-lg border p-4 text-sm">
            <div className="font-medium">整理完成</div>
            <div className="mt-1 text-muted-foreground">
              成功移动 {moved} 个，跳过 {failed} 个。
            </div>
            {results.some((r) => r.reason !== undefined) && (
              <ul className="mt-2 max-h-40 space-y-1 overflow-y-auto text-xs text-muted-foreground">
                {results
                  .filter((r) => r.reason !== undefined)
                  .map((r) => (
                    <li key={r.from}>
                      {baseName(r.from)}：{r.reason}
                    </li>
                  ))}
              </ul>
            )}
          </div>
        )}

        {error !== null && (
          <p className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {error}
          </p>
        )}

        <DialogFooter>
          {results === null ? (
            <>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button
                disabled={busy || plan === null || plan.toMove === 0}
                onClick={() => setConfirmOpen(true)}
              >
                执行整理
              </Button>
            </>
          ) : (
            <Button onClick={() => onOpenChange(false)}>完成</Button>
          )}
        </DialogFooter>

        <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>确认移动文件？</AlertDialogTitle>
              <AlertDialogDescription>
                将移动 {plan?.toMove ?? 0} 个视频到目标目录，并写入
                NFO。此操作会改动磁盘上的文件。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
              <AlertDialogAction disabled={busy} onClick={() => void apply()}>
                确认执行
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DialogContent>
    </Dialog>
  );
}
