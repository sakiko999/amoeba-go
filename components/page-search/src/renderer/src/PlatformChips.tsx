/** 平台选择 chips：「全部」+ Search.Sources 发现的各平台；选中即限定下一次查询。 */

import type { Source } from "./api";
import { platformName } from "./platforms";
import { PlatformIcon } from "platform-icons";

interface PlatformChipsProps {
  sources: Source[];
  selected: string | null; // null = 全部
  onSelect: (namespace: string | null) => void;
}

function Chip({
  active,
  namespace,
  label,
  onClick,
  testId,
}: {
  active: boolean;
  namespace: string;
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
      <PlatformIcon namespace={namespace} />
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
        namespace="all"
        label="全部"
        onClick={() => onSelect(null)}
      />
      {sources.map((src) => (
        <Chip
          key={src.namespace}
          testId={`chip-${src.namespace}`}
          active={selected === src.namespace}
          namespace={src.namespace}
          label={platformName(src.namespace, src.label)}
          onClick={() => onSelect(src.namespace)}
        />
      ))}
    </fieldset>
  );
}
