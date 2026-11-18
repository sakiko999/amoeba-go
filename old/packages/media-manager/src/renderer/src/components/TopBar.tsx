/**
 * 44px 顶栏 = 标题栏（整条 drag 区，右侧留出原生 overlay 窗口按钮宽度）。
 * 交互元素须显式 no-drag，否则 drag 区会吞掉点击。
 */

import { FolderSearch, RefreshCw } from "lucide-react";
import { Button, Input } from "@demo/ui";

interface TopBarProps {
  root: string | null;
  count: number;
  loading: boolean;
  query: string;
  onQuery(query: string): void;
  onPick(): void;
  onRefresh(): void;
}

export function TopBar({
  root,
  count,
  loading,
  query,
  onQuery,
  onPick,
  onRefresh,
}: TopBarProps): React.JSX.Element {
  return (
    <header
      className="flex h-11 shrink-0 items-center gap-2 border-b bg-card px-3"
      style={
        {
          WebkitAppRegion: "drag",
          paddingRight:
            "calc(100vw - env(titlebar-area-x, 0px) - env(titlebar-area-width, 100vw))",
        } as React.CSSProperties
      }
    >
      <Button
        variant="ghost"
        size="sm"
        className="h-8 gap-1.5 [-webkit-app-region:no-drag]"
        onClick={onPick}
      >
        <FolderSearch className="size-4" />
        {root === null ? "选择媒体库" : "切换"}
      </Button>
      <span
        className="min-w-0 flex-1 truncate text-xs text-muted-foreground"
        title={root ?? ""}
      >
        {root ?? "未设置媒体库"}
      </span>
      <Input
        value={query}
        onChange={(e) => onQuery(e.target.value)}
        placeholder="搜索标题…"
        className="h-8 w-52 [-webkit-app-region:no-drag]"
      />
      <Button
        variant="ghost"
        size="sm"
        className="h-8 w-8 p-0 [-webkit-app-region:no-drag]"
        aria-label="刷新"
        disabled={root === null || loading}
        onClick={onRefresh}
      >
        <RefreshCw className={`size-4 ${loading ? "animate-spin" : ""}`} />
      </Button>
      <span className="w-12 text-right text-xs tabular-nums text-muted-foreground">
        {count} 部
      </span>
    </header>
  );
}
