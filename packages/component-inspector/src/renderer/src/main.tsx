import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import type { HostState } from "@host/sdk";
import { App } from "./App";
import "./styles.css";

/** 宿主主题 → Tailwind dark 类（页面内所有 shadcn 变量随之切换） */
function applyHostTheme(theme: HostState["theme"]): void {
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
  root.dataset.theme = theme;
}

const hostState = window.hostState;
if (hostState !== undefined) {
  applyHostTheme(hostState.initial.theme);
  hostState.onChanged((state) => applyHostTheme(state.theme));
}

// bridge 的 onChanged 无 unsubscribe：订阅只挂一次，App 经 window 事件响应
window.componentInspector?.onChanged(() => {
  window.dispatchEvent(new CustomEvent("host:components-changed"));
});

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
