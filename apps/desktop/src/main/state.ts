/**
 * 宿主全局状态（HostState）：唯一状态源 + 持久化 + 订阅。
 * 目前只有 theme；新增状态时扩展 HostState 与 set() 的校验即可，广播层无需改动。
 * 写入用户数据目录的 host-state.json（小文件同步写，简单可靠）；坏文件回退默认值。
 */

import { app } from "electron";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import type { HostState, HostStatePatch } from "@tabs/sdk";

const DEFAULT_STATE: HostState = { theme: "dark" };

type Listener = (state: Readonly<HostState>) => void;

export class HostStateStore {
  private state: HostState = { ...DEFAULT_STATE };
  private readonly listeners = new Set<Listener>();

  private get file(): string {
    return join(app.getPath("userData"), "host-state.json");
  }

  /** app ready 后调用；坏文件/非法字段回退默认，不阻断启动 */
  load(): void {
    try {
      const raw = JSON.parse(
        readFileSync(this.file, "utf8"),
      ) as Partial<HostState>;
      if (raw.theme === "dark" || raw.theme === "light") {
        this.state = { ...this.state, theme: raw.theme };
      }
    } catch {
      // 首次启动没有文件，保持默认
    }
  }

  get(): Readonly<HostState> {
    return this.state;
  }

  /** 返回是否真的发生了变化（无变化不广播、不写盘） */
  set(patch: HostStatePatch): boolean {
    const next: HostState = { ...this.state, ...patch };
    if (next.theme === this.state.theme) return false;
    this.state = next;
    this.persist();
    for (const listener of this.listeners) listener(this.state);
    return true;
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private persist(): void {
    try {
      writeFileSync(
        this.file,
        `${JSON.stringify(this.state, null, 2)}\n`,
        "utf8",
      );
    } catch (err) {
      console.warn("persist host state failed:", err);
    }
  }
}
