# Runtime 页面架构评审

日期：2026-11-06
范围：`apps/desktop` 的 runtime 页面系统、`packages/page-sdk`、`packages/media-manager` 的独立/嵌入双形态。

## 结论

**整体架构 7/10。方向正确，定位是「少量、可信、内置/一等方页面 + 热插拔」，不是第三方页面平台。**

分层是干净的：`PageRegistry` 管页面包生命周期、`TabManager` 管 WebContentsView/布局/菜单、
`HostStateStore` 管全局状态；静态 tab 与 runtime 页面最终汇合到同一套 tab 生命周期，主进程是
唯一权威源。安全基本盘扎实：`page://` 用 `relative + realpath` 校验、按 session 注册、sandbox /
contextIsolation / 权限全拒。media-manager 用同一份 backend 同时跑独立应用与宿主页面，
证明契约能承载真实应用；独立 asar 分发也已通过打包后 `--smoke` 验收。

两条扩展路径成熟度不同：

- **3–5 个可信内置页面：现在就能舒适支持**，主要成本是每个交互页面重复造渲染桥。
- **10+ 页面之前必须先补 P0 四项**，否则复杂度会以「每页一份胶水 + 生命周期偶发故障」线性膨胀。
- **对外第三方页面生态：当前架构不建议**，除非做进程隔离（utilityProcess 级别）。

## 架构优点

1. 双路径汇合：静态 tab 与 runtime 页面共用 `TabManager` 的 mount/activate/close/layout，
   没有两套并行的 tab 生命周期。
2. 依赖方向正确：`PageRegistry` 不认识 tab/view；页面动作由 `PageActions` 编排，
   `apps/desktop/src/main/index.ts` 最终接线。
3. 页面包校验扎实：id 小写 DNS label、entry/preload/main 相对路径 + realpath 双重包含检查、
   扩展名白名单。
4. main 扩展可安装可卸载：`ctx.ipc` tracker 清理、CJS + `require.cache` 失效支持热重载、
   `dataDir` 与宿主 userData 隔离。
5. 状态管理单一来源：`HostStateStore` + 持久化 + 广播；页面 preload 首帧同步取初始值，
   smoke 有端到端主题透传断言。

## 主要风险

### P0（扩到 10+ 页面之前必须做）

| 问题             | 说明                                                                                                                                                                                    | 位置                                                                                   |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| 渲染侧契约缺失   | 页面自带 preload 会整体替换宿主 preload，拿不到 `window.tab`（identity/setTitle/close）与统一的状态桥；每个交互页面重复造桥                                                             | `apps/desktop/src/main/tabs.ts`、`packages/media-manager/src/preload/index.ts`         |
| 生命周期无串行化 | install/reload/uninstall 是 fire-and-forget；并发 reload 产生重复模块实例，`ipcMain.handle` 冲突后 tracker 按 channel 名互删 handler                                                    | `apps/desktop/src/main/pages/actions.ts`、`registry.ts`                                |
| 启动时序错误     | `activateAll()` 在窗口创建、session 策略注册之前；activate 期间 `getWindow()` 必为 undefined，页面新建 session 会绕过 `page://` 与权限拒绝；无超时、main 失败让纯 renderer 页面也打不开 | `apps/desktop/src/main/index.ts`、`registry.ts`                                        |
| IPC 信任边界不足 | 页面 IPC 是全局扁平命名空间；`tabs:*` 没有 sender 角色校验；来源校验只在 media-manager 手写                                                                                             | `pages/registry.ts`、`src/main/index.ts`、`packages/media-manager/src/main/backend.ts` |

### P1

- 卸载不持久、无法升级：重启后自动重新扫描并执行 main.js；同 id 目标存在即拒绝安装。
- `PageMainContext` 太窄：没有 `openTab(route?)`、tab 打开/关闭事件、按 sender 的 setTitle。
- 工具链绑定 media-manager：pack/sync/pages:prepare 硬编码；无页面模板、无嵌入 HMR。
- 宿主无打包管线；`app://` / `page://` 仍用 `net.fetch(file://)`，asar 下未验证。
- 静态 tab 双注册源：`TAB_KINDS` 在 `registry.ts` 与 `electron.vite.config.ts` 两处手写；
  README 的「3 步新增 tab」不成立。
- 可测试性：`apps/desktop` 零单测；smoke 只验加载与主题，且跳过 `state=error` 页面。

### P2

- `HostState.set()` 只比较 `theme`；新增字段忘改会静默不广播、不落盘。
- `page://` 按 session 注册一次却服务所有页面；`corsEnabled: true` 下跨页资源可读，
  与「每页一 origin」的隔离意图不一致。
- `before-quit` 里 `void pages.dispose()`，异步 deactivate 可能被进程退出截断。
- media-manager 纵深防御：`organize:*` 的库根来自 renderer；`image:data-url` 可读任意路径；
  独立 `--smoke` 不隔离 userData。

## 扩展性评分

| 维度                  | 分数 | 说明                                                   |
| --------------------- | ---- | ------------------------------------------------------ |
| 整体架构              | 7/10 | 分层与安全默认扎实；渲染契约、生命周期保护、分发是短板 |
| 新增静态 tab          | 5/10 | 实际要改 4 处，文档写的 3 步不成立                     |
| 新增内置 runtime 页面 | 6/10 | 页面包零宿主改动、可热重载；工具链硬编码、无 HMR       |
| 新增第三方页面        | 2/10 | main.js 全权限、无升级/权限/API 版本，SDK 不可发布     |
| 5–30 页面规模         | 4/10 | 菜单平铺、全量串行激活、扁平 IPC 会随数量放大          |
| 宿主打包              | 2/10 | 无配置、asar 未验证、pages 在 out 之外                 |
| 独立/嵌入双形态       | 8/10 | 同一 backend 支撑两形态，独立 asar 已验收              |
| 可测试性              | 3/10 | 关键生命周期路径无回归保护                             |

## 路线图

### P0

1. **渲染侧统一 bridge**：用 Electron 45 的 `session.registerPreloadScript` 叠加宿主 preload
   （不改页面自有 preload），暴露 `window.tab` 与 `window.hostState`；media 的手写桥改为
   条件注入（独立形态保留，嵌入形态交给宿主）。
2. **IPC 命名空间与来源鉴权**：`ctx.ipc` 校验 `senderFrame.url` 属于 `page://<id>/`；
   `tabs:*` / `tab:*` 复用 `whoami` 做角色校验；重复 channel 给可诊断报错。
3. **启动时序与失败隔离**：先建窗口、注册 session 策略，再懒激活/后台激活页面 main；
   每页 activate 加超时；main 失败时 renderer 仍可打开并展示原因。
4. **安装/卸载/升级语义**：持久化 enabled/disabled；staging + rename 支持同 id 覆盖升级；
   install/reload/uninstall 走串行队列，拆卸期间禁止创建新 tab。

### P1

- 扩展 `PageMainContext`：`openTab(pageId, route?)`、tab 打开/关闭事件、按 sender 的 `setTitle`。
- 抽通用页面工具链：`@pages/cli create/pack/validate` + 模板 + devUrl（嵌入 HMR）。
- 宿主打包：packager/asar + 打包后 smoke；`app://`/`page://` 改 `readFile`；单实例。
- vite input 从 `TAB_KINDS` 生成，消除双注册源。
- `@tabs/sdk`（原 @pages/sdk 已并入）产出可发布的 JS+d.ts；manifest 增加 `apiVersion/minHostVersion`。
- 补 registry/protocol/manifest 单测与并发 reload 集成测试；smoke 把 `state=error` 视为失败。

### P2

- 菜单分组/搜索/图标；HostState 按 key diff 与统一校验；协议响应安全头与按 session 限定页面。
- 退出流程 await `pages.dispose()`；media 库根改为主进程持有并 realpath 校验；
  独立 smoke 隔离 userData。

## 明确不做

- 不开放不可信第三方页面生态（marketplace / 签名 / 权限模型）：`main.js` 在宿主主进程
  全权限执行，真要做得先做 utilityProcess 进程隔离，是产品级投入。
- 不为想象中的场景做 capability proxy、页面间 broker、多窗口 API、通用 settings schema。
- 不重写 `TabManager` / `PageRegistry`，不引入插件框架 / DI / 状态库——现有分层是对的，
  问题在契约边界与生命周期保护。

## 当前决定

先落地 P0 第 1 项（渲染侧统一 bridge）与第 3 项（启动时序与失败隔离），验收后再评估
IPC 鉴权与安装/升级队列。

## 实施进展（2026-11-06）

- P0-1 已落地：新增 `pageBridge` preload，通过 `session.registerPreloadScript` 叠加到页面
  session；页面（含自带 preload 的 media-manager）获得 `window.tab` 与 `window.hostState`，
  media 的 hostState 桥改为仅在独立形态注入。smoke 断言两者同时存在。
- P0-3 已落地：启动顺序改为「建窗 + session 策略 → 加载 host UI → 后台激活页面 main」，
  host UI 165ms、页面激活 171ms（本机 smoke 数据）；每页 activate 8s 超时并回滚；
  main 失败的页面仍可打开 renderer，错误在菜单与安装/重载对话框中提示。
- 诊断增强：smoke 输出 `[startup]` 与每个 tab 的加载耗时；`library:scan` 超过 100ms 时打印
  条目数与耗时；`scanLibrary` 改为 8 路有界并发（大库首屏扫描）。
- 仍待办：P0-2（IPC 命名空间/来源鉴权）、P0-4（安装/卸载/升级语义）。
