# home

宿主首页（`component.json` kind=page，名称「首页」）：列出当前可打开的子页面、
系统环境与组件概览。宿主启动时默认打开本页（未安装/未激活时回退第一个可用页面组件）。
renderer 是 React 19 + Tailwind 4 + @demo/ui（与 media-manager 同一套页面形态）。

## 形态

- **无 main.js**：页面不持有后端状态；数据来自宿主只读 IPC。
- 自有 preload（`src/preload/index.ts`）注入 `window.homeNav`：
  - `list()` → `NAV_CHANNELS.list`（`nav:list`）：所有非 inactive 的页面组件
    （与 `TabManager.create` 的可达语义一致；error 页面仍可打开用于诊断）
  - `open(id)` → `NAV_CHANNELS.open`（`nav:open`）：sender 必须是 tab 角色，
    id 必须是已安装的页面组件，由主进程创建并激活 tab
  - `components()` / `onComponentsChanged()` → 复用 `COMPONENT_CHANNELS`
    （与组件检查页同一份只读数据；安装/卸载/重载后自动刷新清单与概览，
    App 经 window 事件 `host:components-changed` 响应——bridge 订阅只挂一次）
- renderer（`src/renderer/src/App.tsx`）：
  - 子页面卡片（色点 = tab 颜色，静态/页面组件/版本/异常徽标），点击打开
  - 系统环境面板：`window.tab.systemInfo()`（平台/CPU/内存/Electron/Chromium/Node）
  - 组件概览：组件/页面/服务/服务声明计数 + 异常组件列表
  - 主题：`window.hostState.initial`（sendSync 首帧前应用）→ Tailwind `dark` 类
- `window.hostState` 的类型是 `@host/sdk` 的 `HostStateBridge`（media-library 同源）。

## 命令（monorepo 根）

```bash
bun run --cwd packages/home build   # electron-vite（renderer + preload）
bun run --cwd packages/home pack    # build + 组装 out/page/（renderer/ + preload/ + component.json）
```

`bun run pages:prepare` 会打包本页；smoke 断言子页面清单完整且 `open('clock')`（同为页面
组件的 Clock tab）可用。

renderer 构建走 vendor:// 共享层：pack 时 react/@demo/ui external + import map 注入（manifest 缺失自动回退全内联），dev 不受影响。
