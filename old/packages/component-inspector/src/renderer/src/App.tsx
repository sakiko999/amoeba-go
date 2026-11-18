/**
 * 组件依赖页：分层依赖图（provider → consumer，边标注服务名）+ 组件清单。
 *
 * 数据来自宿主只读 IPC（COMPONENT_CHANNELS.list）；依赖解析与分层见 ./graph。
 * 悬停节点时高亮相邻边；inject 找不到提供方显示为「缺失依赖」。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ComponentInfo } from "@host/sdk";
import {
  Badge,
  Button,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@demo/ui";
import { RefreshCw } from "lucide-react";
import { buildGraph, kindLabel, rank, sourceLabel } from "./graph";
import type { GraphModel } from "./graph";

interface EdgePath {
  d: string;
  from: string;
  to: string;
  service: string;
}

const chip =
  "rounded-md px-1.5 py-px text-[11px] whitespace-nowrap";

export function App(): React.ReactElement {
  const api = window.componentInspector;
  const [components, setComponents] = useState<ComponentInfo[] | null>(null);
  const [issue, setIssue] = useState<string | null>(null);
  const [updated, setUpdated] = useState("");

  const model = useMemo(() => buildGraph(components ?? []), [components]);

  const reload = useCallback(async (): Promise<void> => {
    if (api === undefined) {
      setIssue("componentInspector bridge missing — preload 未运行");
      return;
    }
    try {
      setComponents(await api.list());
      setUpdated(new Date().toLocaleTimeString());
    } catch (err) {
      setIssue(`读取组件清单失败：${String(err)}`);
    }
  }, [api]);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    const handler = (): void => {
      void reload();
    };
    window.addEventListener("host:components-changed", handler);
    return () => window.removeEventListener("host:components-changed", handler);
  }, [reload]);

  if (api === undefined) {
    return (
      <main className="p-6 text-sm text-destructive">
        componentInspector bridge missing — preload 未运行
      </main>
    );
  }

  const list = components ?? [];
  const pages = list.filter((c) => c.kind === "page").length;
  const serviceDecls = list.reduce((sum, c) => sum + c.provides.length, 0);
  const errors = list.filter((c) => c.state === "error");
  const tableRows = [...list].sort(
    (a, b) => rank(a) - rank(b) || a.id.localeCompare(b.id),
  );

  return (
    <main className="mx-auto max-w-6xl space-y-6 p-6">
      <header className="flex flex-wrap items-baseline gap-4">
        <h1 className="text-lg font-semibold">组件依赖</h1>
        <div className="flex flex-wrap gap-4 text-sm text-muted-foreground">
          <span>{components !== null ? `${list.length} 个组件` : "…"}</span>
          <span>
            {components !== null
              ? `${pages} 页面 / ${list.length - pages} 服务`
              : "…"}
          </span>
          <span>
            {components !== null ? `${serviceDecls} 个服务声明` : "…"}
          </span>
          <span>{components !== null ? `${model.edges.length} 条依赖` : "…"}</span>
          {updated !== "" && <span>更新于 {updated}</span>}
        </div>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={() => void reload()}
        >
          <RefreshCw /> 刷新
        </Button>
      </header>

      {(errors.length > 0 || model.missing.length > 0) && (
        <div className="space-y-1">
          {errors.map((c) => (
            <p key={c.id} className="text-xs text-destructive">
              组件异常：{c.id}
              {c.error !== undefined ? ` — ${c.error}` : ""}
            </p>
          ))}
          {model.missing.map((miss) => (
            <p key={`${miss.from}:${miss.service}`} className="text-xs text-amber-600">
              缺失依赖：{miss.from} 需要 {miss.service}（无提供方）
            </p>
          ))}
        </div>
      )}

      {issue !== null && <p className="text-xs text-destructive">{issue}</p>}

      <p className="text-xs text-muted-foreground">
        箭头 = 服务依赖（提供方 → 消费方），边上标注服务名；无依赖的组件排在最左列。
        依赖来自各组件的 component.json（provides / inject）。
      </p>

      <DependencyGraph components={list} model={model} />

      <section className="space-y-2">
        <h2 className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
          组件清单
        </h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>组件</TableHead>
              <TableHead>版本</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>来源</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>提供</TableHead>
              <TableHead>依赖</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tableRows.map((c) => (
              <TableRow key={c.id} data-component-row={c.id}>
                <TableCell>
                  <div className="flex flex-col">
                    <span className="font-medium">{c.name}</span>
                    <code className="text-xs text-muted-foreground">{c.id}</code>
                  </div>
                </TableCell>
                <TableCell>{c.version ?? "—"}</TableCell>
                <TableCell>{kindLabel(c.kind)}</TableCell>
                <TableCell>{sourceLabel(c.source)}</TableCell>
                <TableCell>
                  <Badge
                    variant={c.state === "error" ? "destructive" : "outline"}
                    className={
                      c.state === "active"
                        ? "text-emerald-600"
                        : c.state === "error"
                          ? undefined
                          : undefined
                    }
                    title={c.error}
                  >
                    {c.state}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {c.provides.map((service) => (
                      <span
                        key={service.name}
                        className={`${chip} bg-primary/10 text-primary`}
                        title={
                          service.expose !== undefined
                            ? `expose: ${service.expose.join(", ")}`
                            : undefined
                        }
                      >
                        {service.name}
                      </span>
                    ))}
                    {c.provides.length === 0 && (
                      <span className={`${chip} bg-muted text-muted-foreground`}>
                        —
                      </span>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {c.inject.map((service) => {
                      const known = model.providerByService.has(service);
                      return (
                        <span
                          key={service}
                          className={
                            known
                              ? `${chip} bg-muted text-muted-foreground`
                              : `${chip} border border-destructive text-destructive`
                          }
                          title={
                            known
                              ? undefined
                              : "没有组件提供该服务（宿主激活会报 missing）"
                          }
                        >
                          {service}
                        </span>
                      );
                    })}
                    {c.inject.length === 0 && (
                      <span className={`${chip} bg-muted text-muted-foreground`}>
                        —
                      </span>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>
    </main>
  );
}

function DependencyGraph({
  components,
  model,
}: {
  components: ComponentInfo[];
  model: GraphModel;
}): React.ReactElement {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const nodeRefs = useRef(new Map<string, HTMLElement>());
  const [paths, setPaths] = useState<EdgePath[]>([]);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [focusId, setFocusId] = useState<string | undefined>(undefined);

  useEffect(() => {
    const container = containerRef.current;
    if (container === null) return;
    const draw = (): void => {
      const next: EdgePath[] = [];
      for (const edge of model.edges) {
        const from = nodeRefs.current.get(edge.from);
        const to = nodeRefs.current.get(edge.to);
        if (from === undefined || to === undefined) continue;
        const x1 = from.offsetLeft + from.offsetWidth;
        const y1 = from.offsetTop + from.offsetHeight / 2;
        const x2 = to.offsetLeft;
        const y2 = to.offsetTop + to.offsetHeight / 2;
        if (x2 <= x1) continue; // 同层/回边不画线，节点 chip 仍可诊断
        const dx = Math.max(28, (x2 - x1) * 0.45);
        next.push({
          d: `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`,
          from: edge.from,
          to: edge.to,
          service: edge.service,
        });
      }
      setPaths(next);
      setSize({ w: container.scrollWidth, h: container.scrollHeight });
    };
    draw();
    const observer = new ResizeObserver(draw);
    observer.observe(container);
    return () => observer.disconnect();
  }, [model]);

  const maxLayer = components.reduce(
    (max, c) => Math.max(max, model.layer.get(c.id) ?? 0),
    0,
  );
  const sorted = [...components].sort(
    (a, b) =>
      (model.layer.get(a.id) ?? 0) - (model.layer.get(b.id) ?? 0) ||
      rank(a) - rank(b) ||
      a.id.localeCompare(b.id),
  );

  return (
    <div
      ref={containerRef}
      className="relative flex items-start gap-14 overflow-x-auto p-1 pb-4"
      onMouseOver={(event) => {
        const target = (
          event.target as HTMLElement
        ).closest<HTMLElement>("[data-component-id]");
        setFocusId(target?.dataset.componentId);
      }}
      onMouseLeave={() => setFocusId(undefined)}
    >
      <svg
        className="pointer-events-none absolute top-0 left-0 overflow-visible"
        width={size.w}
        height={size.h}
        viewBox={`0 0 ${size.w} ${size.h}`}
        aria-hidden="true"
      >
        <defs>
          <marker
            id="ci-arrow"
            viewBox="0 0 8 8"
            refX={7}
            refY={4}
            markerWidth={7}
            markerHeight={7}
            orient="auto-start-reverse"
          >
            <path
              d="M 0 0 L 8 4 L 0 8 z"
              className="fill-[var(--chart-3)]"
            />
          </marker>
        </defs>
        {paths.map((p) => (
          <path
            key={`${p.from}--${p.to}`}
            d={p.d}
            fill="none"
            strokeWidth={1.4}
            markerEnd="url(#ci-arrow)"
            className={`stroke-[var(--chart-3)] transition-opacity ${
              focusId !== undefined && p.from !== focusId && p.to !== focusId
                ? "opacity-10"
                : "opacity-60"
            }`}
          >
            <title>{`${p.from} → ${p.to}: ${p.service}`}</title>
          </path>
        ))}
      </svg>

      {Array.from({ length: maxLayer + 1 }, (_, layer) => (
        <section key={layer} className="flex w-56 shrink-0 flex-col gap-2.5">
          <h2 className="text-[11px] font-semibold tracking-widest text-muted-foreground uppercase">
            {layer === 0 ? "无依赖" : `第 ${layer} 层`}
          </h2>
          <div className="flex flex-col gap-2.5">
            {sorted
              .filter((c) => (model.layer.get(c.id) ?? 0) === layer)
              .map((c) => (
                <NodeCard key={c.id} component={c} model={model} nodeRefs={nodeRefs} />
              ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function NodeCard({
  component,
  model,
  nodeRefs,
}: {
  component: ComponentInfo;
  model: GraphModel;
  nodeRefs: React.RefObject<Map<string, HTMLElement>>;
}): React.ReactElement {
  const missing = component.inject.filter(
    (service) => !model.providerByService.has(service),
  );
  const stateBorder =
    component.state === "error"
      ? "border-l-red-500"
      : component.state === "active"
        ? "border-l-emerald-500"
        : "border-l-muted-foreground/60";

  return (
    <article
      ref={(el) => {
        if (el === null) nodeRefs.current.delete(component.id);
        else nodeRefs.current.set(component.id, el);
      }}
      data-component-id={component.id}
      data-state={component.state}
      data-missing={missing.length > 0 ? "true" : "false"}
      className={`rounded-lg border border-l-2 bg-card p-3 transition-opacity ${stateBorder} ${
        missing.length > 0 ? "ring-1 ring-destructive" : ""
      }`}
    >
      <div className="flex flex-wrap items-baseline gap-2">
        <span
          className="size-2 shrink-0 rounded-full"
          style={{ backgroundColor: component.color }}
        />
        <span className="text-sm font-semibold">{component.name}</span>
        <code className="text-[11px] text-muted-foreground">
          {component.id}
        </code>
      </div>
      <div className="mt-1.5 flex flex-wrap gap-1.5">
        <Badge variant="secondary">{kindLabel(component.kind)}</Badge>
        <Badge variant="outline">{sourceLabel(component.source)}</Badge>
        <Badge
          variant={component.state === "error" ? "destructive" : "outline"}
          className={component.state === "active" ? "text-emerald-600" : undefined}
          title={component.error}
        >
          {component.state}
        </Badge>
        {component.version !== undefined && (
          <Badge variant="outline">v{component.version}</Badge>
        )}
      </div>
      {component.provides.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {component.provides.map((service) => (
            <span
              key={service.name}
              className={`${chip} bg-primary/10 text-primary`}
              title={
                service.expose !== undefined
                  ? `expose: ${service.expose.join(", ")}`
                  : undefined
              }
            >
              {service.name}
            </span>
          ))}
        </div>
      )}
      {component.inject.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {component.inject.map((service) => {
            const known = model.providerByService.has(service);
            return (
              <span
                key={service}
                className={
                  known
                    ? `${chip} bg-muted text-muted-foreground`
                    : `${chip} border border-destructive text-destructive`
                }
                title={
                  known
                    ? undefined
                    : "没有组件提供该服务（宿主激活会报 missing）"
                }
              >
                {service}
              </span>
            );
          })}
        </div>
      )}
    </article>
  );
}
