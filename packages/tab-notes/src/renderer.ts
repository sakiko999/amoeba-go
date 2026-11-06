/**
 * Notes tab：textarea 草稿 + 把字数同步进 tab 条标题（tab → main → host 联动）。
 * 草稿存在 tab 实例专属的内存 partition localStorage 里，随 tab 关闭即清（demo 不做持久化）。
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
const storageKey = `tab-notes:${api.instanceId}`;

root.innerHTML = `
  <style>
    .wrap {
      height: 100vh;
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      padding: 24px;
      gap: 12px;
      background: var(--bg);
      color: var(--fg);
      font: 14px/1.6 system-ui, "Segoe UI", sans-serif;
    }
    textarea {
      flex: 1;
      resize: none;
      padding: 16px;
      border-radius: 12px;
      border: 1px solid var(--border);
      background: var(--surface);
      color: var(--fg);
      font: inherit;
      outline: none;
    }
    textarea:focus { border-color: var(--accent); }
    footer { color: var(--dim); font-size: 12px; font-family: ui-monospace, Consolas, monospace; }
  </style>
  <div class="wrap">
    <textarea id="editor" placeholder="写点什么——标题会实时显示字数，草稿按 instanceId 持久化"></textarea>
    <footer id="foot"></footer>
  </div>`;

const editor = dom<HTMLTextAreaElement>("#editor");
const foot = dom<HTMLElement>("#foot");

editor.value = localStorage.getItem(storageKey) ?? "";

function persist(): void {
  localStorage.setItem(storageKey, editor.value);
  const count = editor.value.replace(/\s/g, "").length;
  api.setTitle(count > 0 ? `Notes · ${count} 字` : "Notes");
  foot.textContent = `saved · ${new Date().toLocaleTimeString("zh-CN", { hour12: false })} · ${api.instanceId}`;
}

let timer: number | undefined;
editor.addEventListener("input", () => {
  window.clearTimeout(timer);
  timer = window.setTimeout(persist, 300);
});

persist();
