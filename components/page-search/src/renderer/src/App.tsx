/**
 * 聚合搜索页:SearchBar + 平台 chips + 按平台分组的结果区。
 * 流式:Search.Query 每平台完成推一条分组,边到边填;新搜索/取消/切 scope
 * 经世代计数 + channel.close() 使旧调用取消且旧回调失效;卸载同样取消。
 * 分组渲染按 Sources 声明序(权威快照),未到达的平台在 searching 中显示骨架。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Skeleton } from "@demo/ui";
import * as api from "./api";
import type { PlatformGroup, QueryParams, Scope } from "./api";
import { platformName } from "./platforms";
import { PlatformChips } from "./PlatformChips";
import { ResultGroup } from "./ResultGroup";
import { SearchBar } from "./SearchBar";

type Phase = "idle" | "searching" | "done";

function upsertGroup(
  groups: PlatformGroup[],
  next: PlatformGroup,
): PlatformGroup[] {
  const idx = groups.findIndex((g) => g.namespace === next.namespace);
  if (idx < 0) return [...groups, next];
  const copy = groups.slice();
  copy[idx] = next;
  return copy;
}

export function App(): React.ReactElement {
  const [keyword, setKeyword] = useState("");
  const [scope, setScope] = useState<Scope>("live");
  const [sources, setSources] = useState<api.Source[] | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [groups, setGroups] = useState<PlatformGroup[]>([]);
  const [phase, setPhase] = useState<Phase>("idle");
  const [issue, setIssue] = useState<string | null>(null);

  const genRef = useRef(0);
  const cancelRef = useRef<(() => void) | null>(null);

  /** 使在途回调失效并取消 Go 侧剩余平台;保留已到达分组 */
  const stopSearch = useCallback((): void => {
    genRef.current += 1;
    cancelRef.current?.();
    cancelRef.current = null;
    setPhase((p) => (p === "searching" ? "done" : p));
  }, []);

  const doSearch = useCallback(
    (opts: { scope: Scope; platform: string | null }): void => {
      const kw = keyword.trim();
      if (kw === "") return;
      stopSearch();
      const gen = genRef.current;
      const params: QueryParams = {
        keyword: kw,
        scope: opts.scope,
        ...(opts.platform !== null ? { platforms: [opts.platform] } : {}),
      };
      setIssue(null);
      setPhase("searching");
      setGroups([]);
      const handle = api.query(params, (group) => {
        if (gen !== genRef.current) return;
        setGroups((prev) => upsertGroup(prev, group));
      });
      cancelRef.current = handle.cancel;
      handle.done
        .then((result) => {
          if (gen !== genRef.current) return;
          setGroups(result.groups); // Sources 序权威快照
          setPhase("done");
        })
        .catch((err: unknown) => {
          if (gen !== genRef.current) return;
          setIssue(`搜索失败：${String(err)}`);
          setPhase("done");
        });
    },
    [keyword, stopSearch],
  );

  const onScopeChange = useCallback(
    (next: Scope): void => {
      stopSearch();
      setScope(next);
      setSelected(null);
      setGroups([]);
      setPhase("idle");
    },
    [stopSearch],
  );

  const onSelect = useCallback(
    (ns: string | null): void => {
      setSelected(ns);
      if (phase !== "idle") doSearch({ scope, platform: ns });
    },
    [phase, scope, doSearch],
  );

  const loadSources = useCallback(async (sc: Scope): Promise<void> => {
    setSources(null);
    try {
      setSources(await api.sources(sc));
    } catch (err) {
      setIssue(`平台列表加载失败：${String(err)}`);
    }
  }, []);

  useEffect(() => {
    void loadSources(scope);
  }, [scope, loadSources]);

  useEffect(() => cancelRef.current?.(), []); // 卸载:取消在途查询

  if (window.mygo === undefined) {
    return (
      <main className="p-6 text-sm text-destructive">
        window.mygo 未注入 —— 本页需要在 mygo 宿主内打开
      </main>
    );
  }

  const visibleSources = (sources ?? []).filter(
    (src) => selected === null || src.namespace === selected,
  );
  const searching = phase === "searching";
  const totalItems = groups.reduce((sum, g) => sum + g.items.length, 0);
  const allFailed =
    phase === "done" &&
    groups.length > 0 &&
    groups.every((g) => g.error !== undefined);

  return (
    <main className="mx-auto max-w-5xl space-y-4 p-6">
      <h1 className="text-lg font-semibold">聚合搜索</h1>
      <SearchBar
        keyword={keyword}
        onKeywordChange={setKeyword}
        scope={scope}
        onScopeChange={onScopeChange}
        searching={searching}
        onSearch={() => doSearch({ scope, platform: selected })}
        onCancel={stopSearch}
      />
      {sources !== null && sources.length > 0 && (
        <PlatformChips
          sources={sources}
          selected={selected}
          onSelect={onSelect}
        />
      )}
      {issue !== null && <p className="text-xs text-destructive">{issue}</p>}

      {phase === "idle" ? (
        <p className="py-16 text-center text-sm text-muted-foreground">
          输入关键词后 Enter,并行搜索全部平台
        </p>
      ) : (
        <div className="space-y-6">
          {phase === "done" && totalItems === 0 && (
            <p className="text-sm text-muted-foreground">
              {allFailed ? "全部平台搜索失败,见各分组错误" : "没有找到相关结果"}
            </p>
          )}
          {visibleSources.map((src) => {
            const group = groups.find(
              (g) => g.namespace === src.namespace,
            );
            if (group === undefined) {
              return searching ? (
                <div
                  key={src.namespace}
                  data-testid={`pending-${src.namespace}`}
                  className="space-y-2"
                >
                  <p className="text-sm font-semibold">
                    {platformName(src.namespace, src.label)} 搜索中…
                  </p>
                  <Skeleton className="h-24 w-full rounded-xl" />
                </div>
              ) : null;
            }
            return (
              <ResultGroup key={src.namespace} group={group} scope={scope} />
            );
          })}
        </div>
      )}
    </main>
  );
}
