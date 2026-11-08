/** 组件依赖图模型：依赖解析与分层（纯前端计算，与宿主激活语义一致）。 */

import type { ComponentInfo } from "@host/sdk";

export interface Edge {
  from: string;
  to: string;
  service: string;
}

export interface GraphModel {
  /** 服务名 → 提供方组件 id（同名服务宿主只允许一个提供方） */
  providerByService: Map<string, string>;
  edges: Edge[];
  missing: Array<{ from: string; service: string }>;
  /** 组件 id → 依赖层级（无依赖 = 0） */
  layer: Map<string, number>;
}

export function buildGraph(components: ComponentInfo[]): GraphModel {
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

export function kindLabel(kind: ComponentInfo["kind"]): string {
  return kind === "service" ? "服务" : "页面";
}

export function sourceLabel(source: ComponentInfo["source"]): string {
  return source === "external" ? "外部" : "内置";
}

export function rank(component: ComponentInfo): number {
  return component.kind === "service" ? 0 : 1;
}
