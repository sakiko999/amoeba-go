/**
 * 首页：可打开的子页面卡片 + 系统环境 + 组件概览。
 *
 * 子页面来自宿主 NAV_CHANNELS.list（所有非 inactive 的页面组件），
 * 点击经 NAV_CHANNELS.open 由主进程校验后创建 tab；组件概览复用
 * COMPONENT_CHANNELS.list，安装/卸载/重载后经 host:components-changed 自动刷新。
 */

import { useCallback, useEffect, useState } from "react";
import type { ComponentInfo, NavEntry, SystemInfo } from "@host/sdk";
import { Badge, Button, Card, CardContent, CardTitle } from "@demo/ui";
import { RefreshCw } from "lucide-react";

const navGrid =
  "grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(230px,1fr))]";

export function App(): React.ReactElement {
  const nav = window.homeNav;
  const [entries, setEntries] = useState<NavEntry[] | null>(null);
  const [components, setComponents] = useState<ComponentInfo[] | null>(null);
  const [sys, setSys] = useState<SystemInfo | null>(null);
  const [issue, setIssue] = useState<string | null>(null);
  const [updated, setUpdated] = useState("");

  const reload = useCallback(async (): Promise<void> => {
    if (nav === undefined) {
      setIssue("homeNav bridge missing — preload 未运行");
      return;
    }
    try {
      const [nextEntries, nextComponents] = await Promise.all([
        nav.list(),
        nav.components(),
      ]);
      setEntries(nextEntries);
      setComponents(nextComponents);
      setUpdated(new Date().toLocaleTimeString());
    } catch (err) {
      setIssue(`加载失败：${String(err)}`);
    }
  }, [nav]);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    void window.tab
      ?.systemInfo()
      .then(setSys)
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const handler = (): void => {
      void reload();
    };
    window.addEventListener("host:components-changed", handler);
    return () => window.removeEventListener("host:components-changed", handler);
  }, [reload]);

  if (nav === undefined) {
    return (
      <main className="p-6 text-sm text-destructive">
        homeNav bridge missing — preload 未运行
      </main>
    );
  }

  const services = (components ?? []).filter(
    (c) => c.kind === "service",
  ).length;
  const errors = (components ?? []).filter((c) => c.state === "error");
  const serviceDecls = (components ?? []).reduce(
    (sum, c) => sum + c.provides.length,
    0,
  );

  return (
    <main className="mx-auto max-w-6xl space-y-6 p-6">
      <header className="flex flex-wrap items-baseline gap-4">
        <h1 className="text-lg font-semibold">首页</h1>
        <div className="flex flex-wrap gap-4 text-sm text-muted-foreground">
          <span>{entries !== null ? `${entries.length} 个子页面可打开` : "…"}</span>
          <span>
            {components !== null ? `${components.length} 个已安装组件` : "…"}
          </span>
          <span className={errors.length > 0 ? "text-destructive" : undefined}>
            {components !== null
              ? errors.length > 0
                ? `${errors.length} 个异常`
                : "无异常"
              : "…"}
          </span>
          {updated !== "" && <span>更新于 {updated}</span>}
        </div>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={() => void reload()}
        >
          <RefreshCw data-testid="refresh" /> 刷新
        </Button>
      </header>

      {issue !== null && <p className="text-xs text-destructive">{issue}</p>}

      <section className="space-y-3">
        <h2 className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          子页面
        </h2>
        <div className={navGrid}>
          {(entries ?? []).map((entry) => (
            <button
              key={entry.id}
              type="button"
              data-nav-id={entry.id}
              className="flex flex-col gap-2 rounded-xl border bg-card p-4 text-left transition-colors hover:border-primary/60 focus-visible:outline-2 focus-visible:outline-primary"
              onClick={() => {
                void nav
                  .open(entry.id)
                  .then((ok) => {
                    if (ok !== true) setIssue(`无法打开子页面：${entry.id}`);
                  })
                  .catch((err: unknown) =>
                    setIssue(`打开失败：${String(err)}`),
                  );
              }}
            >
              <div className="flex flex-wrap items-baseline gap-2">
                <span
                  className="size-2 shrink-0 rounded-full"
                  style={{ backgroundColor: entry.color }}
                />
                <span className="text-sm font-semibold">{entry.title}</span>
                <code className="text-xs text-muted-foreground">
                  {entry.id}
                </code>
              </div>
              <div className="flex flex-wrap gap-1.5">
                <Badge variant="secondary">页面组件</Badge>
                {entry.version !== undefined && (
                  <Badge variant="outline">v{entry.version}</Badge>
                )}
                {entry.state === "error" && (
                  <Badge variant="destructive" title="main 激活失败；renderer 仍可打开用于诊断">
                    异常
                  </Badge>
                )}
              </div>
            </button>
          ))}
          {entries !== null && entries.length === 0 && (
            <p className="text-sm text-muted-foreground">没有可打开的子页面</p>
          )}
        </div>
      </section>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardContent className="space-y-1">
            <CardTitle className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
              系统环境
            </CardTitle>
            {sys === null ? (
              <p className="text-sm text-muted-foreground">读取中…</p>
            ) : (
              <dl className="text-sm">
                {(
                  [
                    ["平台", `${sys.platform} / ${sys.arch}`],
                    ["CPU", `${sys.cpus} 核`],
                    ["内存", `${sys.memoryGb} GB`],
                    ["Electron", sys.electron],
                    ["Chromium", sys.chrome],
                    ["Node", sys.node],
                  ] as const
                ).map(([label, value]) => (
                  <div
                    key={label}
                    className="flex justify-between gap-3 border-b py-1.5 last:border-b-0"
                  >
                    <dt className="text-muted-foreground">{label}</dt>
                    <dd className="font-mono text-xs">{value}</dd>
                  </div>
                ))}
              </dl>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent className="space-y-3">
            <CardTitle className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
              组件概览
            </CardTitle>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              {(
                [
                  [String(components?.length ?? "—"), "组件"],
                  [
                    String(
                      components !== null ? components.length - services : "—",
                    ),
                    "页面",
                  ],
                  [String(components !== null ? services : "—"), "服务"],
                  [String(components !== null ? serviceDecls : "—"), "服务声明"],
                ] as const
              ).map(([value, label]) => (
                <div
                  key={label}
                  className="flex flex-col gap-0.5 rounded-lg bg-accent p-3"
                >
                  <strong className="text-xl font-semibold text-primary">
                    {value}
                  </strong>
                  <span className="text-xs text-muted-foreground">{label}</span>
                </div>
              ))}
            </div>
            {errors.length > 0 && (
              <p className="text-xs text-destructive">
                异常组件：{errors.map((c) => c.id).join("、")}
              </p>
            )}
            <p className="text-xs text-muted-foreground">
              组件状态与依赖关系详见「组件依赖」页面。
            </p>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
