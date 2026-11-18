/** services.ts 纯逻辑单测：服务表、激活拓扑、依赖反查（bun test） */

import { describe, expect, test } from "bun:test";
import type { ComponentDeps } from "./services";
import { activationOrder, dependentsOf, ServiceTable } from "./services";

const c = (
  id: string,
  kind: "page" | "service",
  provides: string[] = [],
  inject: string[] = [],
): ComponentDeps => ({ id, kind, provides, inject });

describe("ServiceTable", () => {
  test("register/get：同名同提供方可重复登记（幂等）", () => {
    const t = new ServiceTable();
    t.register("media.library", { providerId: "media-library", impl: {} });
    t.register("media.library", { providerId: "media-library", impl: {} });
    expect(t.get("media.library")?.providerId).toBe("media-library");
  });

  test("同名不同提供方抛错（v1 服务名全局唯一）", () => {
    const t = new ServiceTable();
    t.register("media.library", { providerId: "a", impl: {} });
    expect(() =>
      t.register("media.library", { providerId: "b", impl: {} }),
    ).toThrow("服务名冲突");
  });

  test("unregisterByProvider 只清自己的服务", () => {
    const t = new ServiceTable();
    t.register("a.x", { providerId: "a", impl: {} });
    t.register("b.y", { providerId: "b", impl: {} });
    t.unregisterByProvider("a");
    expect(t.get("a.x")).toBeUndefined();
    expect(t.get("b.y")?.providerId).toBe("b");
  });
});

describe("activationOrder", () => {
  test("service 先于 page，页面依赖服务时服务在前", () => {
    const order = activationOrder([
      c("media-manager", "page", [], ["media.library"]),
      c("media-library", "service", ["media.library"]),
    ]);
    expect(order).toEqual(["media-library", "media-manager"]);
  });

  test("服务间链式依赖按拓扑排序", () => {
    const order = activationOrder([
      c("svc-c", "service", ["c.dep"], ["b.dep"]),
      c("svc-b", "service", ["b.dep"]),
      c("svc-a", "service", ["a.dep"], ["c.dep"]),
    ]);
    expect(order.indexOf("svc-b")).toBeLessThan(order.indexOf("svc-c"));
    expect(order.indexOf("svc-c")).toBeLessThan(order.indexOf("svc-a"));
  });

  test("同名服务由两个组件提供直接抛错", () => {
    expect(() =>
      activationOrder([
        c("a", "service", ["dup.x"]),
        c("b", "service", ["dup.x"]),
      ]),
    ).toThrow("服务名冲突");
  });

  test("依赖环抛错并点名环节点", () => {
    expect(() =>
      activationOrder([
        c("a", "service", ["a.out"], ["b.out"]),
        c("b", "service", ["b.out"], ["a.out"]),
      ]),
    ).toThrow("环");
  });

  test("inject 未安装的服务不阻塞排序（激活时 require 报 missing）", () => {
    const order = activationOrder([
      c("orphan-page", "page", [], ["ghost.service"]),
      c("normal-svc", "service", ["ok.service"]),
    ]);
    expect(order).toContain("orphan-page");
    expect(order).toContain("normal-svc");
  });
});

describe("dependentsOf", () => {
  test("反查直接依赖方，排除提供方自身", () => {
    const comps = [
      c("media-library", "service", ["media.library"]),
      c("media-manager", "page", [], ["media.library"]),
      c("clock", "page", [], []),
    ];
    expect(dependentsOf(["media.library"], comps)).toEqual(["media-manager"]);
  });

  test("提供方自己也 inject 另一个同名域服务时不算自身依赖", () => {
    const comps = [
      c("svc", "service", ["a.x"], ["a.y"]),
      c("other", "service", ["a.y"]),
    ];
    expect(dependentsOf(["a.x"], comps)).toEqual([]);
  });
});
