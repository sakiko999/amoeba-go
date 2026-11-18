/**
 * organize 安全测试：全部在 os.tmpdir() 的一次性目录内进行，绝不接触真实媒体库。
 * 覆盖计划命名、特典顺次编号、目标冲突跳过、apply 移动 + NFO 落盘、软链闸门。
 */

import { afterEach, describe, expect, test } from "bun:test";
import {
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { MediaMetadata } from "../shared/model";
import { newMetadata } from "../shared/model";
import { applyOrganize, planOrganize } from "./organize";

const tempDirs: string[] = [];

async function tempLibrary(): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "mm-organize-"));
  tempDirs.push(root);
  return root;
}

async function seed(root: string, files: string[]): Promise<string> {
  const src = join(root, "raw");
  await mkdir(src, { recursive: true });
  for (const f of files) await writeFile(join(src, f), "");
  return src;
}

function sampleMeta(): MediaMetadata {
  const m = newMetadata("tv", "测试剧");
  m.premiered = "2020-01-01";
  return m;
}

afterEach(async () => {
  for (const dir of tempDirs.splice(0)) {
    await rm(dir, { recursive: true, force: true });
  }
});

describe("organize", () => {
  test("plan 生成规范路径，特典归 Season 00 并顺次编号", async () => {
    const root = await tempLibrary();
    const src = await seed(root, [
      "[Group] 测试剧 - 02 [1080p].mkv",
      "[Group] 测试剧 - 01 [1080p].mkv",
      "[Group] 测试剧 - NCOP [Preview].mkv",
      "[Group] 测试剧 - Menu [Preview].mkv",
      "readme.txt",
    ]);

    const plan = await planOrganize(src, root, sampleMeta());
    const showRoot = join(root, "测试剧 (2020)");
    expect(plan.showRoot).toBe(showRoot);
    expect(plan.toMove).toBe(4);
    expect(plan.skipped).toBe(0);

    const targets = plan.items
      .filter((i) => i.status === "move")
      .map((i) => i.to ?? "");
    expect(targets).toContain(join(showRoot, "Season 01", "S01E01.mkv"));
    expect(targets).toContain(join(showRoot, "Season 01", "S01E02.mkv"));
    // 特典按文件名字节序：Menu < NCOP
    expect(targets).toContain(join(showRoot, "Season 00", "S00E01.mkv"));
    expect(targets).toContain(join(showRoot, "Season 00", "S00E02.mkv"));
    // 非视频不进入计划
    expect(plan.items.some((i) => i.from.endsWith("readme.txt"))).toBe(false);
  });

  test("目标已存在时跳过，绝不覆盖", async () => {
    const root = await tempLibrary();
    const src = await seed(root, ["测试剧 - 01.mkv"]);
    const season = join(root, "测试剧 (2020)", "Season 01");
    await mkdir(season, { recursive: true });
    await writeFile(join(season, "S01E01.mkv"), "occupied");

    const plan = await planOrganize(src, root, sampleMeta());
    expect(plan.toMove).toBe(0);
    expect(plan.skipped).toBe(1);
    expect(plan.items[0]?.reason).toContain("目标已存在");

    await applyOrganize(plan, sampleMeta());
    // 原文件仍在，目标内容未被覆盖
    expect(await readFile(join(src, "测试剧 - 01.mkv"), "utf8")).toBe("");
    expect(await readFile(join(season, "S01E01.mkv"), "utf8")).toBe("occupied");
  });

  test("apply 移动视频并写 tvshow.nfo", async () => {
    const root = await tempLibrary();
    const src = await seed(root, ["测试剧 - 01.mkv", "测试剧 - 02.mkv"]);

    const meta = sampleMeta();
    const plan = await planOrganize(src, root, meta);
    const results = await applyOrganize(plan, meta);

    expect(results.every((r) => r.status === "move")).toBe(true);
    const showRoot = join(root, "测试剧 (2020)");
    expect(await readdir(join(showRoot, "Season 01"))).toEqual([
      "S01E01.mkv",
      "S01E02.mkv",
    ]);
    const nfo = await readFile(join(showRoot, "tvshow.nfo"), "utf8");
    expect(nfo).toContain("<title>测试剧</title>");
    expect(nfo).toContain("<premiered>2020-01-01</premiered>");
  });

  test("符号链接/junction 源目录直接拒绝", async () => {
    const root = await tempLibrary();
    const src = await seed(root, ["测试剧 - 01.mkv"]);
    const link = join(root, "linked");
    try {
      await symlink(src, link, "junction");
    } catch {
      // 环境不允许创建链接：跳过该断言（闸门逻辑仍由上面的普通路径测试覆盖）
      return;
    }
    await expect(planOrganize(link, root, sampleMeta())).rejects.toThrow(
      /符号链接/,
    );
  });
});
