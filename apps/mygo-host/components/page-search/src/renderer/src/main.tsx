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

// 首帧前同步应用（pageBridge 注入时刻已带宿主当前主题），避免亮色下闪一帧深色
const hostState = window.hostState;
if (hostState !== undefined) {
  applyHostTheme(hostState.initial.theme);
  hostState.onChanged((state) => applyHostTheme(state.theme));
}

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
