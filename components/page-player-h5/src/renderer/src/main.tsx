import { createRoot } from "react-dom/client";

import { App } from "./App";
import "./styles.css";

// 播放页始终是黑底白字舞台，不跟随宿主主题（与播放器组件本身的配色契约一致），
// 因此不需要 page-search 那套 dark 类切换。

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(<App />);
