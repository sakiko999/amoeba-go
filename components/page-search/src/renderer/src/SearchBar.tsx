/**
 * 搜索条：统一实体搜索的关键词输入 + 搜索/取消按钮。
 * Enter 触发搜索，但跳过中文输入法组合态（isComposing）。
 */

import { Search, X } from "lucide-react";
import { Button, Input } from "@demo/ui";

interface SearchBarProps {
  keyword: string;
  onKeywordChange: (value: string) => void;
  searching: boolean;
  onSearch: () => void;
  onCancel: () => void;
}

export function SearchBar({
  keyword,
  onKeywordChange,
  searching,
  onSearch,
  onCancel,
}: SearchBarProps): React.ReactElement {
  const canSearch = keyword.trim() !== "" && !searching;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Input
        data-testid="search-input"
        className="h-9 min-w-56 flex-1"
        placeholder="搜索实体、频道或播客"
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
