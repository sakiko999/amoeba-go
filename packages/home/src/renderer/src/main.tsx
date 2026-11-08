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

// 首帧前同步应用（pageBridge 已 sendSync 拿到宿主初始状态），避免亮色下闪一帧深色
const hostState = window.hostState;
if (hostState !== undefined) {
  applyHostTheme(hostState.initial.theme);
  hostState.onChanged((state) => applyHostTheme(state.theme));
}

// bridge 的 onChanged 无 unsubscribe：订阅只挂一次，App 经 window 事件响应
window.homeNav?.onComponentsChanged(() => {
  window.dispatchEvent(new CustomEvent("host:components-changed"));
});

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
