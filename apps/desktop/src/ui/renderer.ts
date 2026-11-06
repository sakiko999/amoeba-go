/**
 * Host renderer：Chrome 风格 tab 条（视觉移植自 cherry-studio AppShellTabBar）。
 * 状态仍全部来自主进程（tabs:changed 投影），操作经 window.host 发回；
 * 关闭先本地播 collapse 动画，200ms 后才上报主进程。
 * “+” 与右键菜单走主进程 Menu.popup：host DOM 会被 tab 的原生子视图遮挡。
 * 主题（亮/暗）由主进程广播，这里只切换 <html data-theme> 与按钮图标。
 */

import {
  dom,
  hostApi,
  type TabManifest,
  type TabSummary,
  type Theme,
} from "@tabs/sdk";

const api = hostApi();
const strip = dom<HTMLElement>("#strip");
const empty = dom<HTMLElement>("#empty");
const themeBtn = dom<HTMLButtonElement>("#theme-btn");
const debugBtn = dom<HTMLButtonElement>("#debug-btn");

/** lucide 图标 path（ISC），尺寸对照 cherry-studio：X=10、Plus=14、Sun/Moon=14 */
const X_ICON =
  '<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>';
const PLUS_ICON =
  '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14"/><path d="M12 5v14"/></svg>';
const SUN_ICON =
  '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2"/><path d="M12 20v2"/><path d="m4.93 4.93 1.41 1.41"/><path d="m17.66 17.66 1.41 1.41"/><path d="M2 12h2"/><path d="M20 12h2"/><path d="m6.34 17.66-1.41 1.41"/><path d="m19.07 4.93-1.41 1.41"/></svg>';
const MOON_ICON =
  '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/></svg>';
const BUG_ICON =
  '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m8 2 1.88 1.88"/><path d="M14.2 2 12.3 3.9"/><path d="M9 7.13v-1a3 3 0 1 1 6 0v1"/><path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6"/><path d="M12 20v-9"/><path d="M6.53 9C4.6 9.08 3 10.69 3 12.63V13a2 2 0 0 0 4 0v-1"/><path d="M17.47 9c1.93.08 3.5 1.69 3.5 3.63V13a2 2 0 0 1-4 0v-1"/></svg>';

let tabs: TabSummary[] = [];
let kinds: TabManifest[] = [];
/** 本地正在播关闭动画的 tab：动画结束后才真正 close */
const closingIds = new Set<string>();

const kindOf = (kind: string): TabManifest | undefined =>
  kinds.find((k) => k.id === kind);

function closeTab(tab: TabSummary, el: HTMLElement): void {
  if (closingIds.has(tab.instanceId)) return;
  closingIds.add(tab.instanceId);
  el.classList.add("closing");
  window.setTimeout(() => api.closeTab(tab.instanceId), 200);
}

function renderTab(tab: TabSummary): HTMLButtonElement {
  const el = document.createElement("button");
  el.type = "button";
  el.className = `tab${tab.active ? " active" : ""}${
    closingIds.has(tab.instanceId) ? " closing" : ""
  }`;
  el.role = "tab";
  el.ariaSelected = String(tab.active);
  el.ariaLabel = tab.title;
  el.title = tab.title;

  const dot = document.createElement("span");
  dot.className = "dot";
  dot.style.background = kindOf(tab.kind)?.color ?? "#565f89";

  const label = document.createElement("span");
  label.className = "label";
  label.textContent = tab.title;

  const close = document.createElement("span");
  close.className = "close";
  close.role = "button";
  close.tabIndex = 0;
  close.ariaLabel = "关闭 tab";
  close.innerHTML = X_ICON;
  const doClose = () => closeTab(tab, el);
  close.addEventListener("click", (e) => {
    e.stopPropagation();
    doClose();
  });
  close.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.stopPropagation();
      doClose();
    }
  });

  el.append(dot, label, close);
  // 双击（detail=2）与中键（auxclick button=1）都直接关闭，对照 Chrome/cherry-studio
  el.addEventListener("click", (e) => {
    if (e.detail === 2) return; // 第二击交给 dblclick 处理，避免再 activate
    api.activateTab(tab.instanceId);
  });
  el.addEventListener("dblclick", (e) => {
    e.preventDefault();
    closeTab(tab, el);
  });
  el.addEventListener("auxclick", (e) => {
    if (e.button === 1) {
      e.preventDefault();
      closeTab(tab, el);
    }
  });
  el.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    api.openTabContextMenu(tab.instanceId);
  });
  return el;
}

function renderNewTabButton(): HTMLButtonElement {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "newtab-btn";
  btn.ariaLabel = "新建 tab";
  btn.title = "新建 tab";
  btn.innerHTML = PLUS_ICON;
  btn.addEventListener("click", () => api.openNewTabMenu());
  return btn;
}

function render(): void {
  strip.replaceChildren(...tabs.map(renderTab), renderNewTabButton());
  empty.style.display = tabs.length === 0 ? "flex" : "none";
}

function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
  themeBtn.innerHTML = theme === "dark" ? SUN_ICON : MOON_ICON;
  themeBtn.ariaLabel = theme === "dark" ? "切换亮色主题" : "切换暗色主题";
  themeBtn.title = themeBtn.ariaLabel;
}

themeBtn.addEventListener("click", () => api.toggleTheme());
debugBtn.innerHTML = BUG_ICON;
debugBtn.title = "调试";
debugBtn.addEventListener("click", () => api.openDebugMenu());
api.onThemeChanged(applyTheme);
void api.theme().then(applyTheme);

api.onTabsChanged((next) => {
  // 主进程已移除的 tab 清出动画集，避免重开同名实例时误判 closing
  for (const id of closingIds) {
    if (!next.some((t) => t.instanceId === id)) closingIds.delete(id);
  }
  tabs = next;
  render();
});
const refreshKinds = (): void => {
  void api.listKinds().then((k) => {
    kinds = k;
    render();
  });
};
api.onKindsChanged(refreshKinds);
refreshKinds();
render();
