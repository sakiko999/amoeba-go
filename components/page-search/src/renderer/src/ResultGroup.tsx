/** 一个平台的结果分组：平台名/计数/耗时 + 条目列表；error 显示为分组内提示。 */

import { AlertTriangle } from "lucide-react";
import type { GroupError, PlatformGroup, Scope } from "./api";
import { platformColor, platformName } from "./platforms";
import { ResultItem } from "./ResultItem";

const ERROR_HINTS: Record<string, string> = {
  timeout: "超时",
  canceled: "已取消",
  platform: "平台错误",
  internal: "内部错误",
};

function GroupErrorNote({ error }: { error: GroupError }): React.ReactElement {
  const hint = ERROR_HINTS[error.code] ?? error.code;
  return (
    <p
      data-testid={`group-error`}
      className="flex items-center gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive"
    >
      <AlertTriangle className="size-3.5 shrink-0" />
      <span className="font-medium">{hint}</span>
      <span className="min-w-0 flex-1 truncate opacity-80">{error.message}</span>
    </p>
  );
}

export function ResultGroup({
  group,
  scope,
}: {
  group: PlatformGroup;
  scope: Scope;
}): React.ReactElement {
  const name = platformName(group.namespace, group.label);
  return (
    <section
      data-testid={`group-${group.namespace}`}
      className="space-y-2"
      aria-label={name}
    >
      <header className="flex items-center gap-2 text-sm">
        <span
          className="size-2.5 shrink-0 rounded-full"
          style={{ backgroundColor: platformColor(group.namespace) }}
        />
        <h3 className="font-semibold">{name}</h3>
        <span className="text-xs text-muted-foreground">
          {group.error === undefined
            ? `${group.items.length} 条 · ${group.elapsedMs}ms`
            : `${group.elapsedMs}ms`}
        </span>
      </header>
      {group.error !== undefined && <GroupErrorNote error={group.error} />}
      {group.error === undefined && group.items.length === 0 && (
        <p className="rounded-lg bg-accent px-3 py-2 text-xs text-muted-foreground">
          无匹配结果
        </p>
      )}
      <div className="grid gap-2">
        {group.items.map((item) => (
          <ResultItem
            key={item.id}
            item={item}
            namespace={group.namespace}
            label={group.label}
            scope={scope}
          />
        ))}
      </div>
    </section>
  );
}
