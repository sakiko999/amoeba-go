/**
 * Clock tab：独立 WebContentsView 里的实时时钟。
 * 展示 tab 的独立生命周期：视图创建时启动 interval，关闭 tab 时随 webContents 一起销毁。
 */
import { dom, tabApi, type Theme } from "@host/sdk";

const api = tabApi();
const root = dom<HTMLElement>("#app");

// 主题由主进程广播，本地只切换 <html data-theme>
const applyTheme = (theme: Theme) => {
  document.documentElement.dataset.theme = theme;
};
api.onThemeChanged(applyTheme);
void api.theme().then(applyTheme);

root.innerHTML = `
  <style>
    .wrap {
      height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 8px;
      background: var(--bg);
      color: var(--fg);
      font-family: system-ui, "Segoe UI", sans-serif;
      user-select: none;
    }
    .time { font-size: 64px; font-weight: 200; font-variant-numeric: tabular-nums; letter-spacing: 2px; }
    .date { color: var(--muted); font-size: 15px; }
    .meta { margin-top: 24px; color: var(--dim); font-size: 12px; font-family: ui-monospace, Consolas, monospace; }
  </style>
  <div class="wrap">
    <div class="time" id="time">--:--:--</div>
    <div class="date" id="date"></div>
    <div class="meta" id="meta"></div>
  </div>`;

const time = dom<HTMLElement>("#time");
const date = dom<HTMLElement>("#date");
const meta = dom<HTMLElement>("#meta");
const born = performance.now();

function tick(): void {
  const now = new Date();
  time.textContent = now.toLocaleTimeString("zh-CN", { hour12: false });
  date.textContent = now.toLocaleDateString("zh-CN", { dateStyle: "full" });
  meta.textContent = `instance: ${api.instanceId} · alive: ${((performance.now() - born) / 1000).toFixed(0)}s`;
}

tick();
setInterval(tick, 250);
