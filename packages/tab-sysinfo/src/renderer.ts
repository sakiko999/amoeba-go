/**
 * System tab：主进程聚合的运行环境信息 + IPC 往返延迟演示。
 * 展示 renderer 无法自取、必须经 preload → main 的数据（process.versions 等）。
 */
import { dom, tabApi, type SystemInfo, type Theme } from "@host/sdk";

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
      min-height: 100vh;
      box-sizing: border-box;
      padding: 40px 48px;
      background: var(--bg);
      color: var(--fg);
      font: 14px/1.6 system-ui, "Segoe UI", sans-serif;
    }
    h1 { font-size: 18px; font-weight: 600; margin: 0 0 20px; }
    dl { display: grid; grid-template-columns: max-content 1fr; gap: 8px 24px; margin: 0 0 28px; }
    dt { color: var(--dim); }
    dd { margin: 0; font-family: ui-monospace, Consolas, monospace; }
    button {
      font: inherit;
      padding: 6px 14px;
      border-radius: 8px;
      border: 1px solid var(--accent);
      background: var(--btn);
      color: var(--fg);
      cursor: pointer;
    }
    button:hover { background: var(--surface); }
    button:disabled { opacity: 0.5; cursor: default; }
    #result { margin-top: 12px; color: var(--good); font-family: ui-monospace, Consolas, monospace; min-height: 1.6em; }
  </style>
  <div class="wrap">
    <h1>Runtime</h1>
    <dl id="info"></dl>
    <button id="ping">测一次 IPC 往返（5×ping）</button>
    <div id="result"></div>
  </div>`;

async function main(): Promise<void> {
  const info: SystemInfo = await api.systemInfo();
  const dl = dom<HTMLDListElement>("#info");
  const rows: [string, string][] = [
    ["platform", `${info.platform} / ${info.arch}`],
    ["cpus", String(info.cpus)],
    ["memory", `${info.memoryGb} GB`],
    ["electron", info.electron],
    ["chromium", info.chrome],
    ["node", info.node],
    ["tab instance", `${api.kind} · ${api.instanceId}`],
  ];
  for (const [key, value] of rows) {
    const dt = document.createElement("dt");
    dt.textContent = key;
    const dd = document.createElement("dd");
    dd.textContent = value;
    dl.append(dt, dd);
  }

  const btn = dom<HTMLButtonElement>("#ping");
  const result = dom<HTMLElement>("#result");
  btn.addEventListener("click", async () => {
    btn.disabled = true;
    const samples: number[] = [];
    for (let i = 0; i < 5; i++) {
      const t0 = performance.now();
      const reply = await api.ping(`hello-${i + 1}`);
      samples.push(performance.now() - t0);
      result.textContent = `${i + 1}/5 → ${reply}`;
    }
    const avg = samples.reduce((a, b) => a + b, 0) / samples.length;
    result.textContent = `avg roundtrip: ${avg.toFixed(2)}ms（${samples.map((s) => s.toFixed(1)).join(" / ")}）`;
    btn.disabled = false;
  });
}

void main();
