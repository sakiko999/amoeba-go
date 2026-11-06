/**
 * 已接入 shell 的 tab 清单 —— 全仓库唯一注册点。
 * 新增 tab 子包只需：1) apps/desktop 里 bun add @tabs/xxx；2) 在这里加 import + 数组项。
 * 构建脚本（scripts/build.ts）和 host 的 "+" 菜单都读这一份。
 */
import { manifest as clock } from "@tabs/clock";
import { manifest as notes } from "@tabs/notes";
import { manifest as sysinfo } from "@tabs/sysinfo";
import type { TabManifest } from "@host/sdk";

export const TAB_KINDS: readonly TabManifest[] = [clock, notes, sysinfo];
