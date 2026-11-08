# component-inspector

宿主组件检查页（`component.json` kind=page，名称「组件依赖」）：展示当前安装的
所有组件（页面 + 服务）的 manifest 信息、运行时状态与 provides/inject 依赖图。
renderer 是 React 19 + Tailwind 4 + @demo/ui（与 media-manager 同一套页面形态）。

## 形态

- **无 main.js**：页面不持有后端状态；数据来自宿主只读 IPC。
- 自有 preload（`src/preload/index.ts`）注入 `window.componentInspector`：
  - `list()` → `COMPONENT_CHANNELS.list`（`components:list`），宿主侧为
    `PageRegistry.inspect()`（manifest + state，见 apps/desktop）
  - `onChanged(cb)` ← `COMPONENT_CHANNELS.changed`；组件安装/卸载/重载/级联后
    宿主向所有 tab 广播，App 经 window 事件 `host:components-changed` 重拉
    （bridge 订阅只挂一次，在 main.tsx）
- renderer（`src/renderer/src/App.tsx` + `graph.ts`）：
  - 依赖边 = provider 的 provides 名 → consumer 的 inject 名；找不到提供方显示
    「缺失依赖」（红色 chip，宿主激活时会报 missing）
  - 分层：按 provider→consumer 的最长路径分列（第 0 列 = 无依赖），
    边是叠在 DOM 上的 SVG 曲线（`--chart-3`），悬停节点高亮相邻边
  - 下方清单表格（@demo/ui Table）展示版本、类型、来源、状态、
    provides（title 显示 expose）、inject
  - 主题：`window.hostState.initial`（sendSync 首帧前应用）→ Tailwind `dark` 类

## 命令（monorepo 根）

```bash
bun run --cwd packages/component-inspector build   # electron-vite（renderer + preload）
bun run --cwd packages/component-inspector pack    # build + 组装 out/page/
```

`bun run pages:prepare` 会打包本页；smoke 断言依赖图节点数 = 已安装组件数。

renderer 构建走 vendor:// 共享层：pack 时 react/@demo/ui external + import map 注入（manifest 缺失自动回退全内联），dev 不受影响。
