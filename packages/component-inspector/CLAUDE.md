# component-inspector

宿主组件检查页（`component.json` kind=page，名称「组件依赖」）：展示当前安装的
所有组件（页面 + 服务）的 manifest 信息、运行时状态与 provides/inject 依赖图。

## 形态

- **无 main.js**：页面不持有后端状态；数据来自宿主只读 IPC。
- 自有 preload（`src/preload.ts`）注入 `window.componentInspector`：
  - `list()` → `COMPONENT_CHANNELS.list`（`components:list`），宿主侧为
    `PageRegistry.inspect()`（manifest + state，见 apps/desktop）
  - `onChanged(cb)` ← `COMPONENT_CHANNELS.changed`；组件安装/卸载/重载/级联后
    宿主向所有 tab 广播，页面自动重拉
- renderer（`src/renderer.ts`，无框架）：
  - 依赖边 = provider 的 provides 名 → consumer 的 inject 名；找不到提供方显示
    「缺失依赖」（红色 chip，宿主激活时会报 missing）
  - 分层：按 provider→consumer 的最长路径分列（第 0 列 = 无依赖），
    边是叠在 DOM 上的 SVG 曲线，悬停节点高亮相邻边
  - 下方清单表格展示版本、类型、来源、状态、provides（title 显示 expose）、inject

## 命令（monorepo 根）

```bash
bun run --cwd packages/component-inspector pack   # 产出 out/page/
```

`bun run pages:prepare` 会打包本页；`sync-pages` 投影到宿主
`out/pages/component-inspector`，宿主启动时自动建 tab（smoke 会加载它）。
