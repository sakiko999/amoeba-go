# rss-browser

抓取浏览器组件（`@tabs/rss-browser`）：把 amoeba crawler 的浏览器载具从
WebView2 sidecar（`amoeba-browser.exe`）换成 **Electron 自带 Chromium**。

## 形态

组件 main 在宿主主进程内起一个 **CDP 代理**（`src/proxy.ts`）：

- HTTP：`/json/version`、`/json/list`、`/json/close/:id`、`/json/activate/:id`
- WS browser 端点 `/devtools/browser`：`Target.createTarget` → 新建**隐藏
  BrowserWindow**（session 分区 `persist:rss-crawler`）→ 返回 targetId；
  `Target.closeTarget/getTargets/activateTarget`
- WS page 端点 `/devtools/page/<id>`：任意 CDP 命令 → `webContents.debugger.sendCommand`；
  debugger 事件 → 文本帧广播（lane/超时/资源拦截语义全在 host 侧 CdpBrowserBackend）
- 控制面：`POST /amoeba/close-targets`（空闲回收）、`POST /amoeba/reset-profile`（清分区）

provides `rss.browser`（`endpoint()` → `{port}`、`targetCount()`），
rss-service 组件 inject 它并把端口传给数据服务（`--browser-cdp`）。

## 与 WebView2 sidecar 的差异（有意）

- **UA 洗白**：纯 Chromium UA（`Chrome/<宿主版本>.0.0.0`，无 Electron 标识）——
  Electron UA 段会直接触发强反爬平台风控；
- **全速渲染**：隐藏窗口 `backgroundThrottling:false`，没有 SW_HIDE 停渲染模式；
  滚动驱动（rAF/IntersectionObserver）天然可用，host 的 setMode 为空实现；
- **profile** = Electron session 分区（登录态存宿主 userData，不落 rss dataDir）；
- **安全**：端口只绑 127.0.0.1、无鉴权，与 sidecar CDP 端口同一信任模型（本机）。

## 已知风险（必须真机验证后才能依赖）

指纹不止 UA：canvas/WebGL/字体等与 WebView2 不同；小红书/抖音这类强风控平台
需要真机登录 + 抓取验证。验证命令见 rss-service/CLAUDE.md 的「验证」段。

## 命令（monorepo 根）

```bash
bun run --cwd packages/rss-browser pack   # 产出 out/component/
```

activate 内自检覆盖：createTarget → Network.enable/setBlockedURLs/getAllCookies →
Runtime.evaluate（UA 断言）→ Page.navigate(data:) → evaluate → closeTarget。
