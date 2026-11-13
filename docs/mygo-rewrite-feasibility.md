# 用 mygo 重写桌面宿主的可行性研究

> 2026-11-13。研究对象 [egoist/mygo](https://github.com/egoist/mygo) **v0.9.0**（2026-11-11 release，
> 仓库创建于 2026-09-26，6.1k stars）。结论基于其 docs/docs-zh、源码、官方 examples、
> release notes 与 benchmarks，以及本仓库现状盘点（宿主壳层代码 + `docs/architecture-review.md`）。
> 目标约束：**现有 tab 效果必须尽可能还原**。

## 结论速览

- **架构上可行，且官方示例正好覆盖目标形态**：`examples/embed-webview` 就是「native UI 窗口 +
  多 WebView + tab 切换 + 页面保活」，`examples/frameless` 就是 44px 自绘标题栏。tab 效果还原
  推荐走「chrome webview」路线——**现有 React tab 条代码原样保留**，还原度接近 100%。
- **架构决策（2026-11-13 已定）：service 组件全部改 Go 原生实现，宿主不依赖 bun/任何 JS 运行时**。
  组件管理器与 page 产物形态的规划见 §5.2；代价是 platform-\*（~32k 行）与 media-library 后端
  的 Go 重写成为主要工作量（服务层 2026-11-13 逐文件重估 **≈ 90 人日**，明细见 §7），
  换取体积收益全额兑现（10–20 MB）与最干净的安全模型（宿主进程内再无第三方 JS）。
- **最大非技术风险是成熟度**：项目仅 7 周，两周内连发 0.7.0/0.7.2/0.8.0/0.9.0 四个带 breaking
  changes 的版本，native modules 还标着 experimental。生产采用必须锁版本并持续跟进。
- **另一个缺口**：`browser-proxy` 依赖的 `webContents.debugger`（进程内 CDP）没有等价物，
  爬虫载体已定为「chromedp + spawn 系统浏览器 `--remote-debugging-port`」＋宿主侧 Win32
  隐藏（§5.1.1，headless 已排除、Obscura 维持暂不接入）。
- **收益明确**：发行体积 286.33 MB → 约 10–20 MB（service 组件 Go 化后无 JS 运行时）；内存从
  Electron 常驻（数百 MB）降到官方口径 webview 应用 ~62 MB；自带签名增量更新器与 NSIS 安装器
  （这两样本项目目前都还没做）。
- **建议**：先花约一周做本文 §7 的 PoC 清单（重点是「运行期动态创建/销毁 WebView」和
  「Windows 下自定义 scheme 的 origin 形态」两个未验证点），PoC 通过再决策全量迁移。

## 1. mygo 是什么（v0.9.0 快照）

| 维度 | 事实 |
| --- | --- |
| 定位 | Go 桌面框架；窗口内容二选一或混用：**系统 WebView 的网页** 或 **Go 直接 GPU 绘制的原生 UI** |
| Web 后端 | Windows: WebView2（Evergreen，不固定 Chromium 版本）；macOS: WKWebView（12+）；Linux: WebKitGTK 4.1（4.0 回退，GTK3） |
| IPC | `mygo.Bind` Go 服务 → `mygo generate` 生成 TS 客户端；事件 `NewEvent[T]`、流式 `Channel[T]`、背压 |
| 信任模型 | 只有 trusted page 能调 Go；自定义 scheme 默认 trusted；`TrustedOrigins` 是 origin 级全量信任，**无按方法 ACL** |
| 分发 | 单二进制（前端 embed 后从 `mygo://localhost/` 提供）；NSIS/deb/rpm/dmg；签名 + bsdiff 增量自动更新 |
| 体积/内存 | 官方口径二进制 ~7 MB；native 应用静止 34 MB / webview 应用 ~62 MB（macOS 实测） |
| 成熟度 | 0.x；7 周 1452 commits；0.7→0.9 每版都有 breaking；`docs-zh` 存在 |

## 2. 重写到底要动什么（分层盘点）

先厘清：**这不是"全部重写"**。按层看：

| 层 | 规模 | 迁移命运 |
| --- | --- | --- |
| 宿主壳（`apps/desktop/src/main` + `host-sdk` + 协议/构建胶水） | ~5k 行 TS | **Go 重写**（TabManager→WebView 管理器、Registry/manifest/staging 可机械平移、协议 handler、菜单、状态） |
| 宿主 UI / tab 条（`src/ui/renderer.tsx` + `styles.css`） | ~450 行核心 | **原样保留**，跑在一个专职「chrome webview」里（§3） |
| 页面组件（tab-\*、home、media-manager renderer、rss-subscriptions、`@demo/ui`） | ~13k 行 web | **基本平移**：page 包是预构建静态产物，换协议前缀即可装 |
| media-library 后端（node:sqlite + electron dialog/shell/net） | 2.4k 行 | **Go 重写**（§5.2）：sqlite → pure-Go 驱动（modernc.org/sqlite）优先；dialog/shell 本就是宿主原生能力 |
| platform-\*（12 个平台组件，执行体在 Electron 主进程，electron net 抓取） | ~32k 行 | **Go 重写（主要工作量）**（§5.2）：渠道/客户端/签名（wbi、buvid、弹幕协议）逐平台移植；HTTP 走 Go net/http 或 Fetch 插件 |
| amoeba / rss-service / rss-client（rss 数据栈） | ~17k 行 | **范围外**：当前被排除在打包清单外、未随宿主发行；mygo 版首期不含 rss 数据服务（与 browser-proxy 一并暂缓） |
| `browser-proxy` | 527 行 | **重做**：CDP 载体换成 spawn 的系统浏览器（§5.1） |
| 构建链（pack-components、sync-pages、electron-vite） | scripts | 产物精简（§5.2.2）：renderer 构建保留（target 改 web）、npm production 安装层与共享 node_modules 层删除、宿主构建换 `mygo build` |

> 2026-11-13 修正：早版本曾写 platform-*「本来就在 spawn 的 bun 数据服务进程里、不用动」——
> 经代码核查不成立：12 个 platform-\* 全是 kind=service 组件，main.js 由宿主 `createRequire`
> 拉进 **Electron 主进程**执行（产物实测 `require("electron")`，HTTP 走 `net.fetch`）；
> 真正 spawn bun 进程的只有 rss-service，而 rss 三包目前被 `sync-pages` 排除、未随宿主发行。
> 根 CLAUDE.md 中「rss 平台在网关只注册元数据 API，执行体仍在 rss 数据服务进程」同样是
> 旧架构（提交 6bd849b 之前）的陈旧表述。

关键洞察仍成立但范围要收紧：tab 效果本身是纯 Web（React 19 + Tailwind 4 + dnd-kit + CSS 动画），
**不需要"还原"，只需要让它继续跑**；但「Electron 深耦合代码」不止宿主壳 ~5k 行——platform-\*
与 media-library 的组件 main 也直接依赖 electron（net/dialog/shell），是第二块要处理的深耦合面。
2026-11-13 决策：这块深耦合面按「Go 重写服务」处理，不再保留任何宿主侧 JS（§5.2）。

## 3. tab 效果还原（核心诉求）

### 3.1 现有效果规格（从代码提炼）

`apps/desktop/src/ui/renderer.tsx` + `styles.css`，视觉移植自 cherry-studio AppShellTabBar：

- 44px tab 条即标题栏：`-webkit-app-region: drag` 整条可拖窗口，右侧 `env(titlebar-area-*)`
  留出原生 overlay 窗口按钮区
- tab 本体：30px 高、rounded-10px、彩色圆点（组件色）、标题右缘 20% mask 渐隐、hover 4% /
  active 12% color-mix 背景、active 全亮前景 + 500 字重、focus ring
- 交互动画：dnd-kit 拖拽排序（5px 激活阈值、horizontal strategy、closestCenter、乐观更新）、
  拖拽中浮起 + 0.7 透明度、关闭 200ms collapse（收 0 宽 + 淡出，结束后才真正关）、
  close 钮 hover 展开（宽度 + 透明度 150ms）、tab 间 flex/margin/padding 200ms 让位、
  `prefers-reduced-motion` 支持
- 双击 / 中键关闭、右键原生菜单（关闭/其他/右侧）、"+" sticky 按钮弹原生新建菜单
  （DOM 会被原生子视图遮挡，必须原生 Menu）
- 内容区：`did-finish-load` 前 view 不可见、宿主渲染 spinner 过渡页；view 背景色随主题防暗色
  闪白；**tab 保活**（切走不销毁，瞬时切回）；状态唯一源在主进程、host 只做投影

### 3.2 方案 A：chrome webview（推荐）

窗口是 **native UI 窗口**（mygo 承载 WebView 的硬性要求：`WindowOptions.Content`），结构：

```
native UI 窗口 (Content)
├─ 顶部 44px：chrome webview —— 跑现有 renderer.tsx（React tab 条）
└─ 其余：每 tab 一个 mygo.WebView（ui.WebView 按激活条件构建；未构建 = 隐藏但保活）
```

这正是 `examples/embed-webview`（多 WebView + tab 保活切换）与 `examples/frameless`
（44px 自绘标题栏）两个官方示例的拼装。**tab 条的 DOM/CSS/dnd-kit 原封不动**，
"还原"自然成立。需要改的只有接缝：

| 接缝 | 现状 | mygo 对应 | 状态 |
| --- | --- | --- | --- |
| `window.host` API | `@host/sdk` hostApi | `mygo generate` 生成的 client（窄服务 + 事件） | 改造，量小 |
| 拖拽区 | `-webkit-app-region: drag` | CSS `--app-region: drag`（bridge 内建） | 文档支持，PoC 手感 |
| 窗口按钮区 | `env(titlebar-area-*)` | `--mygo-titlebar-*` 变量 + `TitleBarStyle: hidden`（系统按钮 overlay）+ `SetWindowButtonsTheme` 明暗 | 文档支持，PoC |
| 原生菜单 | `Menu.popup` | mygo 菜单 API（webview 内经绑定服务触发） | ✅ 已验证（原生 Popup，位置=光标） |
| loading 过渡页 | host DOM 占位 | ① native overlay + `loading` 插件 spinner；② 变体 A2（见下）保留在 React | ✅ native UI 状态页 |
| 空 tab 状态 | host DOM | 同上 | ✅ native UI 空态 |
| 防闪白 | `setBackgroundColor` | `BackgroundColor: light-dark(...)` + `Hidden`/`OnReadyToShow` + WebView 自身 `BackgroundColor` | 文档支持 |

**变体 A2（PoC 项）**：让 chrome webview 铺满整窗、内容 WebView 以 `Absolute + ZIndex`
叠在其上方——若 WebView-over-WebView 层叠成立，过渡页/空态也留在 React 里，接缝更少。
文档未明确承诺这种叠法，需要实测。

方案 A 的还原度评估：视觉与动画 100%（同一份代码）；交互手感取决于 WebView2 里
dnd-kit 的 pointer events 表现（PoC 验证）；tab 切换瞬时性由「未构建即隐藏但保活」
的官方语义保证（注意隐藏页默认 `BackgroundThrottling: ThrottlingOn`，需要保活的页面
如时钟/直播弹幕要显式 `ThrottlingOff`）。

### 3.3 方案 B：Go 原生 tab 条（可行但还原度打折，不推荐）

mygo 原生 UI 工具包有 flexbox/grid、spring 动画、SVG 图标（Lucide 直接可用）、明暗主题、
拖放原语（`Draggable`/`Drop`/`ListState.Reorder`），理论上能拼出 tab 条。但：

- **没有现成的可拖拽排序 tab 组件**（内置 `Tabs` 只管选择与可访问性）；dnd-kit 的
  激活阈值、让位动画、命中插入位、5px 位移区分点击/拖拽，全要手搓
- 200ms collapse 关闭动画、标题 mask 渐隐、color-mix 交互态配色要在 Go 侧逐个复刻
- 文本/主题/焦点环与 `@demo/ui` 的像素级一致性靠人肉对齐，且此后**两套 UI 永久分叉维护**

只有当「窗口顶栏必须原生渲染」（如 Glass 材质、极端启动性能）成为硬需求时才值得考虑。

## 4. 能力映射总表

| Electron 能力（现状用法） | mygo 对应 | 判定 |
| --- | --- | --- |
| WebContentsView/tab 叠加 | `Window.NewWebView` + `ui.WebView` 条件构建（隐藏保活） | ✅ 官方示例覆盖 |
| 运行期动态开/关 tab | `NewWebView`/`Destroy`（示例只在启动期创建） | ⚠️ API 存在，需 PoC |
| `persist:page-<id>` session 分区 | `PageOptions.Session` + `NewSession(id)`（文档明言 comparable to Electron partitions） | ✅（macOS 12/13 不落盘） |
| `session.registerPreloadScript` 叠加 pageBridge | `PageOptions.PreloadScript`（页面脚本前运行，`window.mygo` 已就绪，默认仅顶层 frame） | ✅ |
| `page://` 协议（路径穿越双校验） | `Protocol.Handle(scheme, handler)`（自写校验）；**Windows 下 URL 形态为 `http://<scheme>.localhost/`** | ⚠️ 前缀/origin 改写 |
| `vendor://` 共享层 + ACAO | 第二个 scheme；跨 scheme CORS 有文档支持，handler 可写响应头 | ✅ |
| import map 注入 | 构建期注入（现状本就是构建插件插入） | ✅ 不变 |
| CSP + sha256 | 无内建 API；handler 自写 CSP 响应头，hash 逻辑沿用构建脚本 | ✅ 自己做（注意 `Page.Eval` 会绕过页面 CSP） |
| 拒绝导航 / window.open | `OnWillNavigate`（可取消）+ `ContentRules` + `SetWindowOpenHandler`（返回 nil 拒绝）+ `Untrusted` | ✅（Linux 上 JS 弹窗到不了 handler） |
| `Menu.popup` 原生菜单 | mygo 菜单 API | ✅ 已验证（Popup 在光标处弹出） |
| `titleBarStyle: hidden` + overlay 按钮配色 | `TitleBarStyle` / `HideWindowButtons` / `SetWindowButtonsTheme` / `--mygo-titlebar-*` | ✅ 官方 frameless 示例 |
| `setBackgroundColor` 防闪白 | `BackgroundColor: light-dark(...)` + `Hidden`/`OnReadyToShow` | ✅ |
| ipcMain/ipcRenderer + 广播 | `Bind` + 生成 client；`Event.Emit/Broadcast/EmitPage`（DOM ready 前暂存，上限 1024） | ✅ |
| IPC 来源鉴权（whoami） | `CallerPage(ctx)` + **component.json 的 inject 字段升格为页面可调服务 ACL**（§5.2.2）；框架本身无 per-origin ACL | ⚠️ 自己做 |
| F12 devtools | 内置 inspector（生产默认关，`MYGO_INSPECTOR=1` 开） | ✅ |
| spawn bun 数据服务进程 | 不再需要：service 组件全部 Go 内置（2026-11-13 决策），宿主无 JS 子进程 | ✅ |
| `node:sqlite`（media-library 后端） | pure-Go 驱动（modernc.org/sqlite，无 cgo）优先；备选 mygo SQLite 插件（native module，experimental） | ⚠️ PoC 选型 |
| `webContents.debugger`（browser-proxy CDP 载体） | **无等价物**（无 CDP/远程调试端口暴露） | ❌ 见 §5.1 |
| electron-vite / asar / pruneRuntime 工具链 | `mygo build`（前端 embed）+ 自带更新器 | ✅ 换链路 |

## 5. 必须解决的五个问题

### 5.1 爬虫载体（最大功能缺口）

`browser-proxy` 的载具是「隐藏 Electron BrowserWindow 池 + `webContents.debugger` 进程内
CDP attach」，对 amoeba crawler 暴露标准 CDP 端点。mygo **不暴露任何 CDP/远程调试端口**
（Windows 内部虽走 DevTools protocol 做 `Eval`，但那是私有通道）。替代路线：

1. **spawn 系统浏览器**（Edge/Chrome，`--remote-debugging-port` + 独立 profile + 自定义 UA），
   amoeba 侧从「连 browser-proxy」改回「连本地浏览器 CDP」——上游 tauri-rss 本就是这种形态，
   amoeba 改动面小。
2. 指纹语义变化：从「app 自带固定版本 Electron Chromium」变成「用户系统的 Evergreen
   Chrome/Edge」。对反爬可能是净利好（更常见的指纹），也可能是净损失（版本不可控、
   依赖用户装了什么）——而小红书/抖音在真机 Electron 上都还没验证过，**这条链路在任何
   壳下都是未验收状态**，不应单独阻塞 mygo 决策，但要排进 PoC。

#### 5.1.1 载具选型与不可见性复查（2026-11-13）

- **CDP 客户端选 chromedp**：活跃维护（v0.20–0.23，typed cdproto v0.157.x，泛型 API）。
  替代项排除：go-rod 自 2024-07 休眠（fork 非直接替换）；playwright-go 附带 Node driver
  sidecar。体积实测（tmp/size-test，strip 后）：chromedp **+~3.1 MB**（3.83 MB）；
  playwright-go +~6.1 MB 二进制另加 ~90 MB driver（node.exe 79.4 MB）。
- **headless=new 排除**（强反爬平台已知不可用）。形态定为：spawn 系统 Edge/Chrome
  `--remote-debugging-port` + 独立 profile，chromedp 驱动。
- **「页面对用户彻底不可见」刚需的落法**：chromedp 只能把窗口摆屏外，藏不了任务栏——
  补齐靠宿主侧 Win32：窗口 `--window-position` 屏外 +
  `--disable-features=CalculateNativeWinOcclusion`（否则 Chromium occlusion 判定隐藏 →
  不发 XHR），宿主再以 `EnumWindows` + `GetWindowThreadProcessId` 定位 Edge 进程树窗口，
  `SetWindowLongPtr` 叠加 `WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE` 摘掉任务栏/Alt-Tab 条目，
  `WinEventHook` 跟踪新弹窗口——约 100 行 `golang.org/x/sys/windows`。
- **renderLease 语义收窄（用户澄清）**：本义是「抓取时页面真实渲染但对用户不可见」
  （interactive 模式＝窗口显示但摆屏外），不是「同一时刻只有一个页面可见」。lane/空闲回收/
  超时自愈全部降级为可选，仅 renderLease 必须：Go 版 = 互斥锁 + `Target.activateTarget`
  （激活排他，先激活再 scroll，否则静默丢帧），页每 fetch、fail-fast，估 1–1.5 人日。
- **签名 JS 执行面（设计结论）**：appHost.js 的签名代码维持 JS 原文执行，不移植成 Go——
  douyin ABogus（18,527 字符可读 SM3+RC4）、webmssdk（492,195 字符混淆 blob，仅弹幕用，
  占 douyin ~8.7k 行的 76%）、douyu crptext（服务端动态下发）、youtube n-param。Go 侧
  执行面三选：goja（倾向排除，纯增体积/维护成本）/ 隐藏 WebView2 `Page.Eval` /
  系统浏览器 CDP `Runtime.evaluate`。
- **Obscura 复查（维持暂不接入）**：Go 接入面干净——控制面即 CDP WebSocket
  （`obscura serve --port 9222 [--stealth]`），chromedp `NewRemoteAllocator` 直连，与
  Edge 载具同构（单二进制 sidecar ~70 MiB，无 Node）。但 CDP 域是子集：无
  `Target.activateTarget`/Emulation，Network 事件面存疑。`-stealth` 分两层：编译期
  BoringSSL/wreq TLS 指纹伪装（默认版 rustls 指纹可被识破；Windows scoop 默认装的是
  非 stealth 变体）＋运行期指纹随机化/3,520 域 tracker 屏蔽——每次导航重播种，登录持久化
  场景直接撞墙。阻塞项仍是引擎保真度（WebGL/canvas/WebSocket/连接级 cookie），与语言
  无关；等上游修复 + 签名 release 后做 weibo-only POC，届时 Go 侧只是载具层多一个
  backend 分支。

### 5.2 组件模型 v2：内置 Go 服务 + 静态页面包（2026-11-13 决策）

> **决策：service 组件全部改 Go 原生实现，宿主不依赖 bun/任何 JS 运行时。** 早先的
> component-host（bun sidecar）方案作废——它保留 JS 组件生态的代价是 +90 MB 运行时，且把
> 「全权限 JS 进程」换成「全权限 JS 进程墙」只是缓解不是消除。

**为什么 JS main 无法平移进 Go（核查结论，决策依据）**：所有组件 main 被宿主 `createRequire`
在主进程同步执行（`registry.ts:378-381`），实打实 `require("electron")`（platform-\* 走
`net.fetch`、media-library 用 dialog/shell/net + `node:sqlite`）；服务协作是同进程对象直连
（`register` 收含 `invoke` 函数的 ops、danmaku 返回 `subscribe` 函数对）；热重载靠 require.cache
模块替换。替代路径逐条不成立（Go plugin 不支持 Windows、yaegi 解释的是 Go、goja 缺
Node/Electron 模块、Wasm 不成熟、WebView 沙箱无宿主权限）。于是只剩两条路：sidecar 保 JS
（已否决）或 **Go 重写服务（本决策）**。

#### 5.2.1 组件管理器（Go 侧）

```
host/components/
  manager.go     扫描 builtin(go:embed)+external(userData/pages) → 校验 → 状态机 → 激活编排
  manifest.go    component.json 校验（移植 manifest.ts 全部规则：id 正则/kind 约束/路径穿越双校验）
  topology.go    provides/inject Kahn 拓扑与级联（移植 services.ts；服务内置后排序仍约束启动顺序）
  registry.go    服务注册表：Go interface 进程内直连（expose 白名单 = 接口方法集，编译期可查）
  pageroot.go    page:// 静态服务：builtin 走 embed.FS、external 走 os.DirFS，按 host=id 路由
```

- **service 组件 = 编译进宿主的 Go 包**。统一接口
  `type Service interface { Manifest() ServiceManifest; Activate(ctx ServiceContext) error; Deactivate() }`，
  ServiceContext 提供 dataDir/log/宿主能力（dialog、shell、菜单、窗口）；内置清单：
  api-gateway、media-library、platform-\*×12。**不再支持外部安装 kind=service**——"完整
  Node.js 权限"的安装警告随之退役，宿主进程内从此没有第三方代码
- **api-gateway 即 Go 路由表**：register 只收可序列化元数据（namespace/label/tags/op 名），
  `call` 在进程内直达各服务的 Go 方法（同进程函数调用，无 RPC）；media-library 按
  `media.metadata` tag 惰性发现渠道的语义原样保留
- **页面调用宿主服务**：页面是 mygo 的 trusted page，经 `window.mygo` 桥调用；Go 侧用
  `CallerPage(ctx)` 解析页面 origin → 组件 → **component.json 的 `inject` 字段升格为该页面
  可调服务的 ACL 白名单**（schema 不变，语义从「注入对象」变「可调服务」；media-manager 现有
  `inject: ["media.library"]` 直接生效）——顺手修掉现状「require 返回完整 impl」的洞
- **install/uninstall/reload 收窄为 page 语义**：复制目录 + 校验 + 重建 WebView + 清 session
  缓存；状态机（inactive/active/error）与「重载/卸载级联关 tab」保留；服务的 reload 概念消失
  （编译期绑定），菜单里服务组件最多剩一个"停用/启用"开关
- **安全模型净收益**：architecture-review 里「第三方页面必须 utilityProcess 隔离」的 P0 前提
  直接消失——现在最坏情况是装了一个恶意**静态页面**（只能在自己的 webview 里渲染、按 inject
  白名单调服务），而不是一段全权限主进程 JS

#### 5.2.2 page 组件产物形态 v2

```
<id>/
  component.json   # id/name/version/kind=page/entry/color + inject（= 服务 ACL）；不再有 main/preload
  renderer/        # vite build 产物（index.html + assets）；import map 与 CSP hash 仍构建期注入
```

- **不再有 main.js**：页面后端逻辑全部由内置 Go 服务承担。media-manager 的 pageEntry/
  bindMediaIpc（页面 main 桥接层）直接删除——其 12 个 renderer channel 映射为 media.library
  的 Go 绑定，页面改经 `window.mygo` 桥调用
- **不再有 preload**：mygo 对 trusted page 在页面脚本前自动注入 `window.mygo`；页面侧用一个
  固定的薄客户端包（进 vendor:// 共享层，替代 `@host/sdk` 的 window.tab/window.host）。
  页面自带 preload 仍可经 `PageOptions.PreloadScript` 配置（可选能力，v1 不需要）
- **npm 依赖层整体消失**：现状「组件 npm 依赖 production 安装 + sync-pages 的
  out/pages/node_modules 共享层去重合并」是为 main.js 服务的——没有 JS main 后，页面 JS 依赖
  要么构建期内联、要么走 vendor:// 共享层；pack-components 删掉 install 步骤，sync-pages
  退化为纯复制投影，`>120KB 体积门禁`保留（防 external 静默失效）
- **双根静态服务**：builtin 页面随宿主 `go:embed` 进二进制（保住"单文件分发"卖点）；external
  页面从 userData/pages 经 `Protocol.Handle` 文件服务。Windows 下 origin 形态为
  `http://<id>.page.localhost/`（§5.3 的 URL 前缀改写仍适用）
- **vendor:// 共享层机制不变**：Bun.build 预构建 + import map + CSP sha256 全在构建期，与宿主
  壳层解耦，原样平移

**迁移代价重排**：主要工作量从「SDK 重设计」变为 **platform-\* ~32k 行的 Go 重写**——wbi
签名、buvid、弹幕二进制协议这类反爬敏感逻辑的移植是全程最大风险项，建议逐平台排期
（tmdb/douban 等纯 HTTP 元数据平台先行，bili/douyin 等签名/登录平台殿后）；media-library
后端 2.4k 行次之（sqlite 选型：modernc.org/sqlite 纯 Go 优先，避开 mygo SQLite 插件的
experimental 风险）。收益：无 JS 运行时、体积 10–20 MB 全额兑现、宿主进程内零第三方代码。

#### 5.2.3 外壳 PoC 落地记录（2026-11-13，apps/mygo-host）

已跑通并验证（temp 探针 + WebView.CapturePage，正式代码只保留加载日志）：

- chrome tab 条（React 19 + @demo/ui + dnd-kit）原样移植：44px、cherry 交互态、
  关闭 collapse、右键/「+」菜单、明暗主题；编译产物内嵌进宿主（embed），页面经
  `/` 由 `SetFrontend` 服务
- 页面组件装载：`Protocol.Handle("page")` + component.json 扫描；clock/notes 页面真实渲染，
  dnd 排序（`Tabs.Move`）、关闭、主题切换（host.state 广播到全部 trusted webview）链路全通
- **页面桥**：`PageOptions.PreloadScript` 即 Electron `registerPreloadScript` 的等价物——
  `window.mygo` 已在该脚本前注入；用它叠加 `window.tab`/`window.hostState`，Notes 的
  `setTitle` 能实时更新 tab 条标题（页面 → Tabs 服务 → tabs.changed → chrome）
- 两个落地期修正：①`mygo.BindAs` 会校验**全部导出方法参数**的 JSON 可编码性，管理器必须经
  构造函数注入（不能有 `Attach(mgr)` 这类方法）；②`--mygo-titlebar-*` CSS 变量不注入
  WebView 页面，按钮区预留改由 `ui.TitleBar()` → `Tabs.Chrome` 绑定方法下发（实测右预留 138px）
- **观感/可操作性修正（同日验证）**：①WebView2 把 Windows 文字缩放当页面 zoom（TextScale
  125% 时 dpr 1.875、chrome 视口只有 35px），chrome 与页面 webview 统一
  `ZoomFactor=1/TextScale` 后 CSS px 与 DIP 一致（dpr 1.5、44px 条完整）；②chrome 只有
  44px 高，`@demo/ui` 的 DropdownMenu/ContextMenu 会被 webview 视口裁掉（点了没反应）——
  `+`/右键菜单改为宿主 `mygo.Menu.Popup`（`Tabs.PopupNewTabMenu`/`PopupTabMenu` 绑定方法）；
  ③内容区改为 native UI 空态/加载页/错误页：首个 `did-finish-load` 前不挂 webview，
  失败页带「重新加载」（`Page.Reload`），主题切换 Invalidate 重绘
- 已知缺口：home/media-manager 等页面还依赖 `window.homeNav`/`window.media` 页面自有 preload
  桥，在 v2 产物（薄客户端统一走 window.mygo）之前会空白；service 组件尚未实现

#### 5.2.4 存量 TS 包去留（2026-11-13 逐包复查）

| 包 | 判定 |
| --- | --- |
| platform-kit | **整包退役**（运行时语义全部进 Go）。唯一例外：`/provider` 子路径（152 行）——若保留 media-manager 独立 Electron 发行形态则必须存活（`packages/media-manager/src/main/backend.ts:18` 起的 provider 工厂依赖链）；随「独立形态是否继续发行」一并定案 |
| platform-contracts | 缩成页面侧类型薄层；真实页面类型在 `packages/media-library/src/shared/types.ts`，页面侧要替换的运行时其实是 `@host/sdk`（→ window.mygo 薄客户端），contracts 不承载运行时 |
| page-vendor | 原样保留（预构建/import map/CSP hash 全在构建期，与宿主壳解耦） |
| platform-kit `pack.ts` | service 产物打包链随服务 Go 化失去全部用户；page 侧打包按 §5.2.2 精简保留 |
| amoeba | 0 工作量，范围外（rss 栈不随宿主发行） |

### 5.3 Windows 下 scheme origin 形态变化

mygo 在 Windows 把自定义 scheme 映射成 `http://<scheme>.localhost/`（Chromium 视
`*.localhost` 为 secure origin）。影响：页面内的绝对 URL、import map 前缀、CSP 来源、
以及 localStorage 的 origin 键（旧 `persist:page-*` 数据按旧 origin 记录，迁移后等于换新家——
库根重选一次即可，CLAUDE.md 已有同款先例）。改造点集中在 pack-components 的 URL 生成，
一次性成本。另注意 WebView2 对自定义 scheme 响应的 1 MB 缓冲 / 媒体 Range 4 MB 裁剪限制，
`page://` 若承载大媒体需实测（loopback HTTP 是官方后备方案）。

### 5.4 平台边界（本项目当前只发 Windows，影响可控）

- **Linux/WebKitGTK 是重灾区**：自定义 scheme 不能播媒体、无 PiP、DMABUF 默认关、
  无圆角、GTK4 不支持——若将来发 Linux 需专项验收
- macOS 12/13 session 不落盘（14+ 才持久）；WKWebView 用了部分私有 API
- Windows 10 需确认 WebView2 Runtime（Win11 自带）；Chromium 版本随 Evergreen 漂移，
  失去 Electron「锁版本」的确定性（UA、渲染行为随用户环境浮动）

### 5.5 成熟度与跟进成本

7 周 0.x 项目，两周四个带 breaking 的 minor；单作者主导（egoist）+ 快速社区增长。
对本项目（个人/演示体量）可接受，但要：锁 go.mod 版本、升级当显式任务、把宿主与 mygo
的接缝收窄在一个薄适配层里（`webview.go`/`session.go` 等 API 面不大，跟进成本可控）。

## 6. 收益对比

| 维度 | Electron 现状 | mygo 预期 |
| --- | --- | --- |
| 发行体积（win x64 免安装目录） | 286.33 MB（已 prune -78MB） | **~10–20 MB**（二进制 ~7–10 + 前端 embed；service 组件 Go 内置、无 JS 运行时；WebView2 系统提供） |
| 常驻内存 | Electron 全家桶（数百 MB 级） | 官方口径 webview 应用 ~62 MB（macOS 实测；Windows WebView2 同量级，PoC 实测） |
| 更新器 / NSIS | 待做（CLAUDE.md 待办） | **自带**（签名 manifest + bsdiff 增量 + Sparkle 式窗口；NSIS 按用户安装） |
| Chromium 确定性 | 锁版本（45.2.3） | Evergreen 漂移 |
| 主进程生态 | Node（组件 main、node:sqlite、npm） | Go（组件 main 失去寄生处 → sidecar 化） |
| 空窗口/原生 UI 场景 | 同样重 | native UI 窗口完全不启动 WebView 进程 |

## 7. 建议路线

### PoC（约 1 周，全部未知项都在这）

1. 以 `examples/embed-webview` 为底：**运行期动态 `NewWebView`/`Destroy`**（开/关 tab）+
   隐藏保活 + `ThrottlingOff` 验证
2. 顶部 44px chrome webview（renderer.tsx 精简版）：`--app-region: drag` 拖拽、
   `--mygo-titlebar-*` 按钮区预留、`SetWindowButtonsTheme` 随主题、`light-dark` 背景防闪白
3. `page://`（`Protocol.Handle`）在 `http://page.localhost/` 形态下跑真实页面组件包 +
   `vendor://` import map + 跨 scheme CORS 实测
4. `NewSession(id)` 的 localStorage 持久化（Windows）与多 tab 同组件多实例隔离
5. dnd-kit 拖拽在 WebView2 的手感（pointer events、5px 阈值、拖拽中浮起）
6. webview 内触发原生菜单 popup；`loading` 插件做过渡 spinner（或验证 A2 的 WebView 叠层）
7. （若爬虫进 PoC）spawn Edge `--remote-debugging-port` + amoeba 真实抓一条 weibo 频道
8. **Go 组件管理器原型 + 首个 Go 服务**（§5.2）：manager 跑通 manifest 校验/拓扑/状态机；
   api-gateway（Go 路由表）+ platform-tmdb（最简纯 HTTP 平台）出 Go 版；media-manager 页面包
   转 v2 产物装进宿主，页面经 `window.mygo` 桥 + inject ACL 调 media.library——打通
   「静态页面包 → 内置 Go 服务」全链路；顺带完成 sqlite 选型（modernc.org/sqlite vs mygo 插件）

### 全量迁移（PoC 通过后；服务层 2026-11-13 逐文件重估）

**服务层 Go 重写 ≈ 90 人日（85–105）**——12 个 platform-\* + api-gateway + kit 运行时
（含 CDP 载具）移植；宿主壳与页面平移不在内。逐块明细：

| 块 | 人日 | 明细 |
| --- | --- | --- |
| 元数据平台 | 9 | tmdb 1、douban 0.5、bangumi 1、apple 1.5、youtube 5 |
| 直播平台 | 37 | bili 15、douyu 8、huya 8、twitch 6 |
| 爬虫平台 | 25 | weibo 3、xhs 9、douyin 13（其中签名执行面 3–5） |
| 基础设施 | 19.5 | api-gateway 1.5、kit 运行时 15（含 CDP 载具 5–8）、contracts 2、vendor 1 |
| amoeba | 0 | 范围外 |

名义 ~32k 行 platform-\* 中**真正可移植约 21k 行**（webmssdk 混淆 blob 与浏览器胶水只能
换执行面，不能「移植」，见 §5.1.1）。不可排期项：xhs/douyin/bili 真机登录与反爬验收；
扫码登录的 gateway op 设计（scanLogin 不在 expose 面）。

1. 宿主骨架：窗口/tab/协议/session/菜单/主题（TabManager 的 Go 版）+ 组件管理器（§5.2.1）
2. 内置服务 Go 化：api-gateway → media-library → 12 个 platform-\*（tmdb/douban 等 HTTP 元数据
   平台先行，bili/douyin 等签名/登录平台殿后——§5.2 末段的最大风险项）
3. 页面组件平移到 v2 产物（§5.2.2：删 main/preload/npm 层，pack 链精简）
4. `mygo build` + 更新器 + NSIS；smoke 改走 automation 插件（Unix socket，非 CDP）

## 8. 判断

- **tab 效果：不需要妥协。** 方案 A 让现有 tab 条代码原样存活，这是本次研究最明确的结论。
- **可行性成立的条件**（2026-11-13 更新）：接受 service 组件全量 Go 重写（~35k 行，主要成本，
  其中平台签名/反爬逻辑的移植是最大风险项）、接受爬虫载体日后换系统浏览器、接受 0.x 跟进
  成本；换来的是无 JS 运行时的 10–20 MB 分发与「宿主进程内零第三方代码」的安全模型。
- **留在 Electron 的最强理由**：爬虫链路若最终证明必须用「自带可控 Chromium + 进程内 CDP」，
  Electron 本身就是资产，mygo 反而是负资产。反之，若主发 Windows、在乎体积/内存/更新器，
  mygo 收益显著。
- 建议按 §7 先做 PoC，把「动态 WebView」「scheme origin」「dnd 手感」「CDP 载体」四个未知项
  变成事实后再拍板。

## 来源

- 官网/文档：mygo.run（docs、showcase、benchmarks）；仓库 `docs/` 与 `docs-zh/`
- 关键源码：`webview.go`、`window.go`、`page.go`、`session.go`、`protocol.go`、`bridge.go`、`content_rules.go`
- 官方示例：`examples/embed-webview`（多 WebView + tab 保活）、`examples/frameless`（44px 自绘标题栏）
- release notes：v0.7.0 → v0.9.0（breaking changes 记录）
- 本仓库：`apps/desktop/src/main/tabs.ts`、`src/ui/renderer.tsx`、`src/ui/styles.css`、
  `pages/protocol.ts`、`vendorProtocol.ts`、`docs/architecture-review.md`、`docs/desktop-release-size.md`
