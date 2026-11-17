/**
 * 搜索条：关键词输入 + scope 切换（搜直播间/搜主播）+ 搜索/取消按钮。
 * Enter 触发搜索，但跳过中文输入法组合态（isComposing）。
 */

import { Search, X } from "lucide-react";
import { Button } from "@demo/ui";
import { Input } from "@demo/ui";
import { Tabs, TabsList, TabsTrigger } from "@demo/ui";
import type { Scope } from "./api";

interface SearchBarProps {
  keyword: string;
  onKeywordChange: (value: string) => void;
  scope: Scope;
  onScopeChange: (scope: Scope) => void;
  searching: boolean;
  onSearch: () => void;
  onCancel: () => void;
}

export function SearchBar({
  keyword,
  onKeywordChange,
  scope,
  onScopeChange,
  searching,
  onSearch,
  onCancel,
}: SearchBarProps): React.ReactElement {
  const canSearch = keyword.trim() !== "" && !searching;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Tabs value={scope} onValueChange={(v) => onScopeChange(v as Scope)}>
        <TabsList>
          <TabsTrigger value="live">直播间</TabsTrigger>
          <TabsTrigger value="channel">主播</TabsTrigger>
        </TabsList>
      </Tabs>
      <Input
        data-testid="search-input"
        className="h-9 min-w-56 flex-1"
        placeholder={
          scope === "live" ? "搜索关键词，如：英雄联盟" : "搜索主播 / 频道名"
        }
        value={keyword}
        autoFocus
        onChange={(e) => onKeywordChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.nativeEvent.isComposing && canSearch) {
            onSearch();
          }
        }}
      />
      {searching ? (
        <Button
          data-testid="search-cancel"
          variant="outline"
          size="sm"
          onClick={onCancel}
        >
          <X /> 取消
        </Button>
      ) : (
        <Button
          data-testid="search-submit"
          size="sm"
          disabled={!canSearch}
          onClick={onSearch}
        >
          <Search /> 搜索
        </Button>
      )}
    </div>
  );
}
