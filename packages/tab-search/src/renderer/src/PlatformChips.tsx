/** 平台选择 chips：「全部」+ Search.Sources 发现的各平台；选中即限定下一次查询。 */

import type { Source } from "./api";
import { platformColor, platformName } from "./platforms";

interface PlatformChipsProps {
  sources: Source[];
  selected: string | null; // null = 全部
  onSelect: (namespace: string | null) => void;
}

function Chip({
  active,
  color,
  label,
  onClick,
  testId,
}: {
  active: boolean;
  color: string;
  label: string;
  onClick: () => void;
  testId?: string;
}): React.ReactElement {
  return (
    <button
      type="button"
      data-testid={testId}
      onClick={onClick}
      className={`flex shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-primary ${
        active
          ? "border-primary bg-primary/10 font-medium text-foreground"
          : "border-border text-muted-foreground hover:border-primary/50 hover:text-foreground"
      }`}
    >
      <span
        className="size-2 shrink-0 rounded-full"
        style={{ backgroundColor: color }}
      />
      {label}
    </button>
  );
}

export function PlatformChips({
  sources,
  selected,
  onSelect,
}: PlatformChipsProps): React.ReactElement {
  return (
    <fieldset
      className="flex min-w-0 gap-2 overflow-x-auto pb-0.5"
      aria-label="平台筛选"
    >
      <Chip
        active={selected === null}
        color="#7aa2f7"
        label="全部"
        onClick={() => onSelect(null)}
      />
      {sources.map((src) => (
        <Chip
          key={src.namespace}
          testId={`chip-${src.namespace}`}
          active={selected === src.namespace}
          color={platformColor(src.namespace)}
          label={platformName(src.namespace, src.label)}
          onClick={() => onSelect(src.namespace)}
        />
      ))}
    </fieldset>
  );
}
