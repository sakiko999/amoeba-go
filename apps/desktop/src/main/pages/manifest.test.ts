/** manifest.ts 校验单测：component.json 回退、kind 矩阵、provides/inject 规则 */

import { afterAll, describe, expect, test } from "bun:test";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { readPageManifest } from "./manifest";

const tmpRoot = join(
  import.meta.dir,
  `.manifest-test-${process.pid.toString(36)}`,
);

async function makePkg(
  name: string,
  manifest: Record<string, unknown>,
  file = "component.json",
  files: Record<string, string> = {},
): Promise<string> {
  const dir = join(tmpRoot, name);
  await mkdir(dir, { recursive: true });
  await writeFile(join(dir, file), JSON.stringify(manifest), "utf8");
  for (const [path, content] of Object.entries(files)) {
    await mkdir(join(dir, path, ".."), { recursive: true });
    await writeFile(join(dir, path), content, "utf8");
  }
  return dir;
}

afterAll(async () => {
  await rm(tmpRoot, { recursive: true, force: true });
});

describe("manifest 读取", () => {
  test("component.json 优先，page.json 回退", async () => {
    const both = await makePkg(
      "both",
      { id: "both", name: "B", entry: "index.html" },
      "component.json",
      { "index.html": "<html></html>" },
    );
    await writeFile(
      join(both, "page.json"),
      JSON.stringify({ id: "legacy", name: "L", entry: "index.html" }),
      "utf8",
    );
    const m = await readPageManifest(both);
    expect(m.id).toBe("both");
    expect(m.kind).toBe("page");

    const legacy = await makePkg(
      "legacy",
      { id: "legacy", name: "L", entry: "index.html" },
      "page.json",
      { "index.html": "<html></html>" },
    );
    expect((await readPageManifest(legacy)).id).toBe("legacy");
  });
});

describe("kind 矩阵", () => {
  const html = { "index.html": "<html></html>" };

  test("service：必填 main，禁止 entry/preload/color", async () => {
    const ok = await makePkg(
      "svc-ok",
      {
        id: "svc-ok",
        name: "S",
        kind: "service",
        main: "main.js",
        provides: [{ name: "svc.ok", expose: ["ping"] }],
      },
      "component.json",
      { "main.js": "exports.activate=()=>{}" },
    );
    const m = await readPageManifest(ok);
    expect(m.kind).toBe("service");
    if (m.kind !== "service") return;
    expect(m.main).toBe("main.js");
    expect(m.provides?.[0]?.expose).toEqual(["ping"]);

    await expect(
      makePkg(
        "svc-no-main",
        { id: "svc-no-main", name: "S", kind: "service" },
        "component.json",
        html,
      ).then(readPageManifest),
    ).rejects.toThrow("必须提供 main");

    await expect(
      makePkg(
        "svc-entry",
        {
          id: "svc-entry",
          name: "S",
          kind: "service",
          main: "main.js",
          entry: "index.html",
        },
        "component.json",
        { "main.js": "", ...html },
      ).then(readPageManifest),
    ).rejects.toThrow("不允许 entry");
  });

  test("page：entry 必填（.html）", async () => {
    await expect(
      makePkg(
        "page-no-entry",
        { id: "page-no-entry", name: "P", kind: "page" },
        "component.json",
        html,
      ).then(readPageManifest),
    ).rejects.toThrow("entry");
  });
});

describe("provides / inject", () => {
  test("非法服务名与 host.* 保留前缀被拒绝", async () => {
    const base = {
      id: "bad-name",
      name: "B",
      kind: "service",
      main: "main.js",
    };
    const files = { "main.js": "" };
    await expect(
      makePkg(
        "bad1",
        { ...base, provides: [{ name: "no-dot" }] },
        "component.json",
        files,
      ).then(readPageManifest),
    ).rejects.toThrow("服务名非法");
    await expect(
      makePkg(
        "bad2",
        { ...base, provides: [{ name: "host.state" }] },
        "component.json",
        files,
      ).then(readPageManifest),
    ).rejects.toThrow("保留前缀");
  });

  test("provides 与 inject 重叠被拒绝；声明 provides 必须有 main", async () => {
    const files = { "main.js": "" };
    await expect(
      makePkg(
        "overlap",
        {
          id: "overlap",
          name: "O",
          kind: "service",
          main: "main.js",
          provides: [{ name: "a.x" }],
          inject: ["a.x"],
        },
        "component.json",
        files,
      ).then(readPageManifest),
    ).rejects.toThrow("重叠");

    await expect(
      makePkg(
        "no-main",
        {
          id: "no-main",
          name: "N",
          kind: "page",
          entry: "index.html",
          provides: [{ name: "a.x" }],
        },
        "component.json",
        { ...files, "index.html": "" },
      ).then(readPageManifest),
    ).rejects.toThrow("必须提供 main");
  });
});
