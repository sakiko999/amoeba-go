/**
 * 首页：可打开的子页面卡片 + 系统环境 + 组件概览。
 *
 * 子页面来自宿主 NAV_CHANNELS.list（静态 tab + 非 inactive 的页面组件），
 * 点击经 NAV_CHANNELS.open 由主进程校验后创建 tab。组件概览复用
 * COMPONENT_CHANNELS.list（与组件检查页同一份数据，含 changed 自动刷新）。
 */

import type { ComponentInfo, NavEntry, SystemInfo, Theme } from "@host/sdk";

interface HomeNav {
  list(): Promise<NavEntry[]>;
  open(id: string): Promise<boolean>;
  components(): Promise<ComponentInfo[]>;
  onComponentsChanged(cb: () => void): void;
}

declare global {
  interface Window {
    homeNav?: HomeNav;
  }
}

const bridge = window.homeNav;
if (bridge === undefined) {
  throw new Error("homeNav bridge missing — preload 未运行");
}
const homeNav: HomeNav = bridge;

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

const summaryEl = must<HTMLElement>("#summary");
const issuesEl = must<HTMLElement>("#issues");
const navEl = must<HTMLElement>("#nav");
const envEl = must<HTMLElement>("#env");
const statsEl = must<HTMLElement>("#stats");

function showIssue(text: string): void {
  issuesEl.hidden = false;
  issuesEl.append(el("div", "issue", text));
}

/* ---------- 主题（宿主 pageBridge 的 window.tab） ---------- */

function applyTheme(theme: Theme | undefined): void {
  document.documentElement.dataset.theme =
    theme === "light" ? "light" : "dark";
}

async function followHostTheme(): Promise<void> {
  const tab = window.tab;
  if (tab === undefined) return;
  applyTheme((await tab.state()).theme);
  tab.onStateChanged((state) => applyTheme(state.theme));
}
void followHostTheme().catch(() => undefined);

/* ---------- 子页面卡片 ---------- */

function renderNav(entries: NavEntry[]): void {
  navEl.replaceChildren();
  for (const entry of entries) {
    const card = el("button", "nav-card");
    card.type = "button";
    card.dataset.navId = entry.id;

    const head = el("div", "nav-head");
    const dot = el("span", "nav-dot");
    dot.style.background = entry.color;
    head.append(dot, el("span", "nav-title", entry.title));
    head.append(el("code", "nav-id", entry.id));
    card.append(head);

    const meta = el("div", "nav-meta");
    meta.append(
      el("span", "badge", entry.origin === "static" ? "静态 tab" : "页面组件"),
    );
    if (entry.version !== undefined) {
      meta.append(el("span", "badge", `v${entry.version}`));
    }
    if (entry.state === "error") {
      const badge = el("span", "badge state-error", "异常");
      badge.title = "main 激活失败；renderer 仍可打开用于诊断";
      meta.append(badge);
    }
    card.append(meta);

    card.addEventListener("click", () => {
      void homeNav
        .open(entry.id)
        .then((ok) => {
          if (ok !== true) showIssue(`无法打开子页面：${entry.id}`);
        })
        .catch((err: unknown) => showIssue(`打开失败：${String(err)}`));
    });
    navEl.append(card);
  }
  if (entries.length === 0) {
    navEl.append(el("div", "empty", "没有可打开的子页面"));
  }
}

/* ---------- 系统环境 ---------- */

async function loadEnv(): Promise<void> {
  const tab = window.tab;
  if (tab === undefined) {
    envEl.closest("section")?.remove();
    return;
  }
  const info = (await tab.systemInfo()) as SystemInfo;
  const rows: Array<[string, string]> = [
    ["平台", `${info.platform} / ${info.arch}`],
    ["CPU", `${info.cpus} 核`],
    ["内存", `${info.memoryGb} GB`],
    ["Electron", info.electron],
    ["Chromium", info.chrome],
    ["Node", info.node],
  ];
  envEl.replaceChildren();
  for (const [label, value] of rows) {
    const row = el("div", "kv-row");
    row.append(el("dt", "", label), el("dd", "", value));
    envEl.append(row);
  }
}
void loadEnv().catch(() => undefined);

/* ---------- 组件概览 ---------- */

function renderStats(components: ComponentInfo[]): void {
  const services = components.filter((c) => c.kind === "service").length;
  const errors = components.filter((c) => c.state === "error");
  const serviceDecls = components.reduce(
    (sum, c) => sum + c.provides.length,
    0,
  );

  const stat = (value: string, label: string): HTMLElement => {
    const box = el("div", "stat");
    box.append(el("strong", "", value), el("span", "", label));
    return box;
  };
  statsEl.replaceChildren(
    stat(String(components.length), "组件"),
    stat(String(components.length - services), "页面"),
    stat(String(services), "服务"),
    stat(String(serviceDecls), "服务声明"),
  );
  if (errors.length > 0) {
    const line = el(
      "div",
      "issue",
      `异常组件：${errors.map((c) => c.id).join("、")}`,
    );
    statsEl.append(line);
  }
}

/* ---------- 数据 ---------- */

async function reload(): Promise<void> {
  const [entries, components] = await Promise.all([
    homeNav.list(),
    homeNav.components(),
  ]);
  renderNav(entries);
  renderStats(components);
  const openable = entries.length;
  const errors = components.filter((c) => c.state === "error").length;
  summaryEl.replaceChildren(
    el("span", "", `${openable} 个子页面可打开`),
    el("span", "", `${components.length} 个已安装组件`),
    errors > 0 ? el("span", "", `${errors} 个异常`) : el("span", "", "无异常"),
    el("span", "", `更新于 ${new Date().toLocaleTimeString()}`),
  );
}

must<HTMLButtonElement>("#refresh").addEventListener("click", () => {
  void reload().catch((err: unknown) => showIssue(`刷新失败：${String(err)}`));
});

homeNav.onComponentsChanged(() => {
  // 安装/卸载/重载后子页面清单与组件概览都可能变化
  void reload().catch((err: unknown) => {
    console.error("home reload failed:", err);
  });
});

void reload().catch((err: unknown) => showIssue(`加载失败：${String(err)}`));
