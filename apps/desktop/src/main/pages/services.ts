/**
 * 组件服务表与激活拓扑（纯逻辑，不依赖 electron，可被 bun test 直接覆盖）。
 * - ServiceTable：全局服务名 → 提供方；同名服务只允许一个提供方，冲突即抛错
 * - activationOrder：按 provides/inject 依赖做拓扑排序（service 优先、环检测），
 *   activateAll 按该顺序激活，保证页面 require 时服务已就绪
 * - dependentsOf：反查依赖某组服务的组件，reload/uninstall 服务组件时级联停用
 */

import type { PageManifest } from "@host/sdk";

export interface ServiceEntry {
  /** 提供方组件 id */
  providerId: string;
  /** 服务实现对象（方法集） */
  impl: object;
  /** 方法名白名单（来自 manifest.provides[].expose，经 ctx.provide 校验一致） */
  expose?: string[];
}

export class ServiceTable {
  private readonly services = new Map<string, ServiceEntry>();

  register(name: string, entry: ServiceEntry): void {
    const existing = this.services.get(name);
    if (existing !== undefined && existing.providerId !== entry.providerId) {
      throw new Error(
        `服务名冲突：${name} 已由 ${existing.providerId} 提供，` +
          `${entry.providerId} 不能重复提供`,
      );
    }
    this.services.set(name, entry);
  }

  get(name: string): ServiceEntry | undefined {
    return this.services.get(name);
  }

  /** 停用/回滚组件时注销它提供的全部服务 */
  unregisterByProvider(providerId: string): void {
    for (const [name, entry] of this.services) {
      if (entry.providerId === providerId) this.services.delete(name);
    }
  }

  names(): string[] {
    return [...this.services.keys()];
  }
}

/** 拓扑排序用的最小投影（registry 从 PageManifest 投影出来） */
export interface ComponentDeps {
  id: string;
  kind: "page" | "service";
  provides: string[];
  inject: string[];
}

export function projectDeps(manifest: PageManifest): ComponentDeps {
  return {
    id: manifest.id,
    kind: manifest.kind ?? "page",
    provides: (manifest.provides ?? []).map((s) => s.name),
    inject: [...(manifest.inject ?? [])],
  };
}

/**
 * 激活顺序：依赖边（B provides s、A inject s ⇒ B 先于 A）的 Kahn 拓扑排序。
 * 无依赖时 service 组件排在 page 前（保证纯服务先就绪）。
 * 环（如 A inject B 的服务、B inject A 的服务）直接抛错。
 */
export function activationOrder(components: ComponentDeps[]): string[] {
  const providers = new Map<string, string>(); // serviceName -> providerId
  for (const c of components) {
    for (const s of c.provides) {
      const prev = providers.get(s);
      if (prev !== undefined && prev !== c.id) {
        throw new Error(`服务名冲突：${s} 同时由 ${prev} 与 ${c.id} 提供`);
      }
      providers.set(s, c.id);
    }
  }

  // depCount[A] = A 等待的服务数（只计有提供方的：缺失服务在激活时由
  // ctx.require 报 missing，不阻塞排序）；dependents[B] = 等 B 的组件
  const depCount = new Map<string, number>();
  const dependents = new Map<string, string[]>();
  for (const c of components) {
    depCount.set(
      c.id,
      c.inject.filter(
        (s) => providers.get(s) !== undefined && providers.get(s) !== c.id,
      ).length,
    );
    for (const s of c.inject) {
      const provider = providers.get(s);
      if (provider === undefined || provider === c.id) continue;
      const list = dependents.get(provider);
      if (list === undefined) dependents.set(provider, [c.id]);
      else list.push(c.id);
    }
  }

  // 无依赖时 service 优先；同优先级保持声明顺序（stable）
  const byId = new Map(components.map((c) => [c.id, c] as const));
  const ready = components
    .filter((c) => (depCount.get(c.id) ?? 0) === 0)
    .sort((a, b) => rank(a) - rank(b) || 0)
    .map((c) => c.id);
  const order: string[] = [];
  while (ready.length > 0) {
    const id = ready.shift();
    if (id === undefined) break;
    order.push(id);
    for (const dep of dependents.get(id) ?? []) {
      const left = (depCount.get(dep) ?? 0) - 1;
      depCount.set(dep, left);
      if (left === 0) {
        ready.push(dep);
        // 维持 service 优先的稳定顺序（规模很小，插入排序足够）
        ready.sort((a, b) => rank(byId.get(a)) - rank(byId.get(b)) || 0);
      }
    }
  }
  if (order.length !== components.length) {
    const cyclic = components
      .filter((c) => !order.includes(c.id))
      .map((c) => c.id);
    throw new Error(`组件服务依赖存在环：${cyclic.join(" → ")}`);
  }
  return order;
}

/** 反查依赖任一服务名的其他组件（不含提供方自身） */
export function dependentsOf(
  serviceNames: string[],
  components: ComponentDeps[],
): string[] {
  const wanted = new Set(serviceNames);
  return components
    .filter((c) => !c.provides.some((s) => wanted.has(s)))
    .filter((c) => c.inject.some((s) => wanted.has(s)))
    .map((c) => c.id);
}

function rank(c: ComponentDeps | undefined): number {
  // service 先于 page：保证页面 activate 时纯服务已注册
  return c?.kind === "service" ? 0 : 1;
}
