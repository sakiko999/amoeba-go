/**
 * ramda 语义基线：ramda 经 Bun.build tree-shake 内联进组件 main.js（devDependencies
 * 运输模式），此文件锁住组件实际依赖的非直观语义。升级 ramda 前后必须保持全绿——
 * 行为变化意味着三个 platform 组件的 bundle 语义同时变化。
 */
import * as R from "ramda";
import { describe, expect, test } from "bun:test";

describe("ramda 语义基线", () => {
  test("maxBy 比较值相等时返回第二个参数", () => {
    // platform-youtube/src/lib/client.ts 的清晰度回退依赖此行为
    const first = { v: 1, tag: "first" };
    const second = { v: 1, tag: "second" };
    expect(R.maxBy((x: { v: number }) => x.v, first, second)).toBe(second);
    expect(R.maxBy((x: { v: number }) => x.v, { v: 0 }, { v: 2 })).toMatchObject({
      v: 2,
    });
  });

  test("pathOr 在路径中途 null/缺失时取默认值", () => {
    expect(R.pathOr("def", ["a", "b"], { a: null })).toBe("def");
    expect(R.pathOr("def", ["a", "b"], {})).toBe("def");
    expect(R.pathOr("def", ["a", "b"], { a: { b: "val" } })).toBe("val");
  });

  test("defaultTo 只替换 null/undefined/NaN", () => {
    expect(R.defaultTo(42, NaN)).toBe(42);
    expect(R.defaultTo(42, null)).toBe(42);
    expect(R.defaultTo(42, 0)).toBe(0);
    expect(R.defaultTo(42, "")).toBe("");
  });

  test("sortWith 依次取首个非零比较器，且不修改原数组", () => {
    const src = [
      { a: 2, b: 1 },
      { a: 1, b: 9 },
      { a: 2, b: 0 },
    ];
    const sorted = R.sortWith(
      [R.ascend((x: { a: number }) => x.a), R.descend((x: { b: number }) => x.b)],
      src,
    );
    expect(sorted).toEqual([
      { a: 1, b: 9 },
      { a: 2, b: 1 },
      { a: 2, b: 0 },
    ]);
    expect(src[0]).toEqual({ a: 2, b: 1 });
  });

  test("常用组合链行为", () => {
    const pick = R.pipe(
      R.filter((x: number) => x % 2 === 1),
      R.map((x: number) => x * 10),
      R.chain((x: number) => [x, x + 1]),
    );
    expect(pick([1, 2, 3])).toEqual([10, 11, 30, 31]);
    expect(R.reduce((acc: number, x: number) => acc + x, 0, [1, 2, 3])).toBe(6);
    expect(R.last([1, 2, 3])).toBe(3);
    expect(R.reverse([1, 2])).toEqual([2, 1]);
    expect(R.findIndex((x: number) => x === 2, [1, 2, 3])).toBe(1);
    expect(R.sortBy((x: { k: string }) => x.k, [{ k: "b" }, { k: "a" }])).toEqual([
      { k: "a" },
      { k: "b" },
    ]);
  });
});
