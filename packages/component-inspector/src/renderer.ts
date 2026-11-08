/**
 * 组件依赖页：从宿主 IPC 取已安装组件（manifest 声明 + 运行时状态），
 * 画分层依赖图（provider → consumer，边标注服务名）并列出组件清单。
 *
 * 依赖解析是纯前端计算：inject 的服务名 → 在已安装组件的 provides 里找提供方；
 * 找不到的注入显示为「未提供」。宿主激活时的拓扑排序见
 * apps/desktop/src/main/pages/services.ts（同一份 provides/inject 语义）。
 */

import type { ComponentInfo, Theme } from "@host/sdk";

interface InspectorBridge {
  list(): Promise<ComponentInfo[]>;
  onChanged(cb: () => void): void;
}

declare global {
  interface Window {
    componentInspector?: InspectorBridge;
  }
}

const bridge = window.componentInspector;
if (bridge === undefined) {
  throw new Error("componentInspector bridge missing — preload 未运行");
}
const inspector: InspectorBridge = bridge;

/* ---------- 主题（宿主 pageBridge 的 window.tab 提供状态订阅） ---------- */

function applyTheme(theme: Theme | undefined): void {
  document.documentElement.dataset.theme = theme === "light" ? "light" : "dark";
}

async function followHostTheme(): Promise<void> {
  const tab = window.tab;
  if (tab === undefined) return;
  applyTheme((await tab.state()).theme);
  tab.onStateChanged((state) => applyTheme(state.theme));
}
void followHostTheme().catch(() => undefined);

/* ---------- DOM 工具 ---------- */

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className !== undefined) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function must<T extends Element>(selector: string): T {
  const found = document.querySelector<T>(selector);
  if (found === null) throw new Error(`element not found: ${selector}`);
  return found;
}

const graphEl = must<HTMLElement>("#graph");
const summaryEl = must<HTMLElement>("#summary");
const issuesEl = must<HTMLElement>("#issues");
const tbody = must<HTMLTableSectionElement>("#components-body");

/** 当前渲染的边高亮函数；监听器只挂一次，重渲染只替换实现。 */
let focusGraph: (id?: string) => void = () => undefined;
graphEl.addEventListener("mouseover", (event) => {
  const target = (event.target as HTMLElement | null)?.closest<HTMLElement>(
    "[data-component-id]",
  );
  focusGraph(target?.dataset.componentId);
});
graphEl.addEventListener("mouseleave", () => focusGraph(undefined));

/* ---------- 依赖模型 ---------- */

interface Edge {
  from: string;
  to: string;
  service: string;
}

interface GraphModel {
  /** 服务名 → 提供方组件 id（同名服务宿主只允许一个提供方） */
  providerByService: Map<string, string>;
  edges: Edge[];
  missing: Array<{ from: string; service: string }>;
  /** 组件 id → 依赖层级（无依赖 = 0） */
  layer: Map<string, number>;
}

function buildGraph(components: ComponentInfo[]): GraphModel {
  const providerByService = new Map<string, string>();
  for (const component of components) {
    for (const service of component.provides) {
      providerByService.set(service.name, component.id);
    }
  }

  const edges: Edge[] = [];
  const missing: Array<{ from: string; service: string }> = [];
  const parents = new Map<string, Set<string>>();
  for (const component of components) {
    for (const service of component.inject) {
      const provider = providerByService.get(service);
      if (provider === undefined) {
        missing.push({ from: component.id, service });
        continue;
      }
      if (provider === component.id) continue;
      edges.push({ from: provider, to: component.id, service });
      let set = parents.get(component.id);
      if (set === undefined) {
        set = new Set();
        parents.set(component.id, set);
      }
      set.add(provider);
    }
  }

  const layer = new Map<string, number>();
  const visiting = new Set<string>();
  const layerOf = (id: string): number => {
    const cached = layer.get(id);
    if (cached !== undefined) return cached;
    if (visiting.has(id)) return 0; // 环由宿主激活拒绝；这里只防渲染死循环
    visiting.add(id);
    let level = 0;
    for (const parent of parents.get(id) ?? []) {
      level = Math.max(level, layerOf(parent) + 1);
    }
    visiting.delete(id);
    layer.set(id, level);
    return level;
  };
  for (const component of components) layerOf(component.id);

  return { providerByService, edges, missing, layer };
}

function kindLabel(kind: ComponentInfo["kind"]): string {
  return kind === "service" ? "服务" : "页面";
}

function sourceLabel(source: ComponentInfo["source"]): string {
  return source === "external" ? "外部" : "内置";
}

function rank(component: ComponentInfo): number {
  return component.kind === "service" ? 0 : 1;
}

/* ---------- 渲染 ---------- */

let graphObserver: ResizeObserver | undefined;

function render(components: ComponentInfo[]): void {
  const model = buildGraph(components);
  renderSummary(components, model);
  renderGraph(components, model);
  renderTable(components, model);
}

function renderSummary(components: ComponentInfo[], model: GraphModel): void {
  const pages = components.filter((c) => c.kind === "page").length;
  const services = components.reduce((sum, c) => sum + c.provides.length, 0);
  const errors = components.filter((c) => c.state === "error").length;

  summaryEl.replaceChildren(
    el("span", "", `${components.length} 个组件`),
    el("span", "", `${pages} 页面 / ${components.length - pages} 服务`),
    el("span", "", `${services} 个服务声明`),
    el("span", "", `${model.edges.length} 条依赖`),
    el("span", "", `更新于 ${new Date().toLocaleTimeString()}`),
  );

  const issues: HTMLElement[] = [];
  if (errors > 0) {
    for (const component of components.filter((c) => c.state === "error")) {
      issues.push(
        el(
          "div",
          "issue",
          `组件异常：${component.id}${component.error !== undefined ? ` — ${component.error}` : ""}`,
        ),
      );
    }
  }
  for (const miss of model.missing) {
    issues.push(
      el(
        "div",
        "issue warn",
        `缺失依赖：${miss.from} 需要 ${miss.service}（无提供方）`,
      ),
    );
  }
  issuesEl.replaceChildren(...issues);
  issuesEl.hidden = issues.length === 0;
}

function renderGraph(components: ComponentInfo[], model: GraphModel): void {
  graphObserver?.disconnect();
  graphEl.replaceChildren();

  const maxLayer =
    components.length === 0
      ? 0
      : Math.max(...components.map((c) => model.layer.get(c.id) ?? 0));
  const columns: HTMLElement[] = [];
  for (let i = 0; i <= maxLayer; i += 1) {
    const column = el("section", "layer");
    column.append(el("h2", "layer-title", i === 0 ? "无依赖" : `第 ${i} 层`));
    const body = el("div", "layer-body");
    column.append(body);
    columns.push(body);
    graphEl.append(column);
  }

  const sorted = [...components].sort(
    (a, b) =>
      (model.layer.get(a.id) ?? 0) - (model.layer.get(b.id) ?? 0) ||
      rank(a) - rank(b) ||
      a.id.localeCompare(b.id),
  );
  const nodeOf = new Map<string, HTMLElement>();
  for (const component of sorted) {
    const node = renderNode(component, model);
    nodeOf.set(component.id, node);
    columns[model.layer.get(component.id) ?? 0]?.append(node);
  }

  // SVG 边层：绝对定位在 graphEl 上（graphEl 是 position: relative）
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "edges");
  svg.setAttribute("aria-hidden", "true");
  const defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
  defs.innerHTML =
    '<marker id="arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 8 4 L 0 8 z" fill="currentColor" /></marker>';
  svg.append(defs);
  graphEl.prepend(svg);

  const edgeEls: Array<{ path: SVGPathElement; edge: Edge }> = [];
  const draw = (): void => {
    const width = graphEl.scrollWidth;
    const height = graphEl.scrollHeight;
    svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
    svg.setAttribute("width", String(width));
    svg.setAttribute("height", String(height));
    for (const path of svg.querySelectorAll("path.edge")) path.remove();
    edgeEls.length = 0;

    for (const edge of model.edges) {
      const from = nodeOf.get(edge.from);
      const to = nodeOf.get(edge.to);
      if (from === undefined || to === undefined) continue;
      const x1 = from.offsetLeft + from.offsetWidth;
      const y1 = from.offsetTop + from.offsetHeight / 2;
      const x2 = to.offsetLeft;
      const y2 = to.offsetTop + to.offsetHeight / 2;
      if (x2 <= x1) continue; // 同层/回边不画线，节点 chip 仍可诊断
      const dx = Math.max(28, (x2 - x1) * 0.45);
      const path = document.createElementNS(
        "http://www.w3.org/2000/svg",
        "path",
      );
      path.setAttribute("class", "edge");
      path.setAttribute(
        "d",
        `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`,
      );
      path.setAttribute("marker-end", "url(#arrow)");
      const title = document.createElementNS(
        "http://www.w3.org/2000/svg",
        "title",
      );
      title.textContent = `${edge.from} → ${edge.to}: ${edge.service}`;
      path.append(title);
      svg.append(path);
      edgeEls.push({ path, edge });
    }
  };
  requestAnimationFrame(draw);
  graphObserver = new ResizeObserver(draw);
  graphObserver.observe(graphEl);

  focusGraph = (id?: string): void => {
    for (const { path, edge } of edgeEls) {
      path.classList.toggle(
        "dim",
        id !== undefined && edge.from !== id && edge.to !== id,
      );
    }
  };
}

function renderNode(component: ComponentInfo, model: GraphModel): HTMLElement {
  const missing = component.inject.filter(
    (service) => !model.providerByService.has(service),
  );
  const node = el("article", "node");
  node.dataset.componentId = component.id;
  node.dataset.state = component.state;
  node.dataset.kind = component.kind;
  node.dataset.missing = String(missing.length > 0);

  const head = el("div", "node-head");
  const dot = el("span", "node-dot");
  dot.style.background = component.color;
  head.append(dot, el("span", "node-name", component.name));
  head.append(el("code", "node-id", component.id));
  node.append(head);

  const meta = el("div", "node-meta");
  meta.append(el("span", "badge", kindLabel(component.kind)));
  meta.append(el("span", "badge", sourceLabel(component.source)));
  const state = el("span", `badge state-${component.state}`, component.state);
  if (component.error !== undefined) state.title = component.error;
  meta.append(state);
  if (component.version !== undefined) {
    meta.append(el("span", "badge", `v${component.version}`));
  }
  node.append(meta);

  if (component.provides.length > 0) {
    const chips = el("div", "chips");
    for (const service of component.provides) {
      const chip = el("span", "chip", service.name);
      if (service.expose !== undefined) {
        chip.title = `expose: ${service.expose.join(", ")}`;
      }
      chips.append(chip);
    }
    node.append(chips);
  }

  if (component.inject.length > 0) {
    const chips = el("div", "chips");
    for (const service of component.inject) {
      const known = model.providerByService.has(service);
      const chip = el("span", known ? "chip ghost" : "chip missing", service);
      if (!known) chip.title = "没有组件提供该服务（宿主激活会报 missing）";
      chips.append(chip);
    }
    node.append(chips);
  }

  return node;
}

function renderTable(components: ComponentInfo[], model: GraphModel): void {
  tbody.replaceChildren();
  const sorted = [...components].sort(
    (a, b) => rank(a) - rank(b) || a.id.localeCompare(b.id),
  );
  for (const component of sorted) {
    const row = document.createElement("tr");
    row.dataset.componentRow = component.id;

    const nameCell = document.createElement("td");
    const nameBox = el("div", "cell-name");
    nameBox.append(el("strong", "", component.name));
    nameBox.append(el("code", "", component.id));
    nameCell.append(nameBox);

    const stateCell = document.createElement("td");
    const state = el("span", `badge state-${component.state}`, component.state);
    if (component.error !== undefined) state.title = component.error;
    stateCell.append(state);

    const providesCell = document.createElement("td");
    const provideChips = el("div", "chips");
    for (const service of component.provides) {
      const chip = el("span", "chip", service.name);
      if (service.expose !== undefined) {
        chip.title = `expose: ${service.expose.join(", ")}`;
      }
      provideChips.append(chip);
    }
    if (component.provides.length === 0) {
      provideChips.append(el("span", "chip ghost", "—"));
    }
    providesCell.append(provideChips);

    const injectCell = document.createElement("td");
    const injectChips = el("div", "chips");
    for (const service of component.inject) {
      const known = model.providerByService.has(service);
      const chip = el("span", known ? "chip ghost" : "chip missing", service);
      if (!known) chip.title = "没有组件提供该服务（宿主激活会报 missing）";
      injectChips.append(chip);
    }
    if (component.inject.length === 0) {
      injectChips.append(el("span", "chip ghost", "—"));
    }
    injectCell.append(injectChips);

    row.append(
      nameCell,
      el("td", "", component.version ?? "—"),
      el("td", "", kindLabel(component.kind)),
      el("td", "", sourceLabel(component.source)),
      stateCell,
      providesCell,
      injectCell,
    );
    tbody.append(row);
  }
}

/* ---------- 数据 ---------- */

async function reload(): Promise<void> {
  const components = await inspector.list();
  render(components);
}

must<HTMLButtonElement>("#refresh").addEventListener("click", () => {
  void reload().catch((err: unknown) => {
    issuesEl.hidden = false;
    issuesEl.replaceChildren(
      el("div", "issue", `读取组件清单失败：${String(err)}`),
    );
  });
});

inspector.onChanged(() => {
  void reload().catch((err: unknown) => {
    console.error("component-inspector reload failed:", err);
  });
});

void reload().catch((err: unknown) => {
  issuesEl.hidden = false;
  issuesEl.replaceChildren(
    el("div", "issue", `读取组件清单失败：${String(err)}`),
  );
});
