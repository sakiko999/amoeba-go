# 扫码登录公开实现调研

> 2026-11-20。调研方式:公开仓库/文档核对 + 当前网络抽样探测。只记录登录协议
> 可行性,不涉及绕过风控。取码与未扫码轮询已实测;除特别注明外,扫码确认后的
> 换票/cookie 落地未做真实手机扫码验证。

## 代码位置

扫码登录的**协议与页面配方**归各平台组件所有，`service-credentials` 只做会话、
凭据镜像与页面绑定：

| 平台 | 登录实现 |
|---|---|
| 微博 | `components/platform-weibo/login_flow.go` / `login_http.go` / `login_open.go` |
| 小红书 | `components/platform-xhs/login.go` |
| 抖音 | `components/platform-douyin/login.go` |
| 豆瓣 | `components/platform-douban/login_flow.go` / `login_http.go` / `login_open.go` |
| B站 | `components/platform-bili/login_flow.go`（协议仍在 `login.go`） |

共用契约在 `platform/login/`（`Flow`、`Session`、`Env` 与 cookie/HTTP/页面工具）；
装配清单在 `services/auth.go`。新增支持扫码的平台时，只需在对应平台组件导出一个
`login.Flow`，再登记到 `services.LoginFlows()`，不改凭据服务。

## 结论

| 平台 | 优先级 | 建议 | 主要原因 |
|---|---:|---|---|
| 快手 | 高 | 下一个接入 | 协议链路短,全部表单请求,2026-11-20 实测取码仍可用 |
| 微博 | 高 | 纯 HTTP 优先,浏览器兜底 | 新版取码可用;旧版接口在当前出口返回 403 |
| B站 | 已完成 | 保持现状 | `generate → poll` 成功时直接下发 `SESSDATA` |
| 豆瓣 | 已完成 | 保持现状 | `qrlogin_code → qrlogin_status`,确认后补访 `stat.html` 与首页 |
| 小红书 | 低 | 保持浏览器 | 必须带 `x-s`/`x-s-common`;确认后常见 HTTP 461/471 验证码 |
| 抖音 | 低 | 保持浏览器 | `a_bogus`、SecSDK CSRF;HTTP 会话缺 verifycenter 信任会遇 `2156` |
| 知乎 | 低 | 暂缓 | 纯 HTTP 曾可通,现易触发 40352 环境验证 |
| 网易云 | 中 | 视需求接入 | weapi 加密清晰,`unikey → client/login` 到 803 成功 |

## 公开实现

### 快手

- `wilsonwussd/KSQRcode`:`kuaishou_login.py`
- 链路:`/rest/c/infra/ks/qr/start` → `scanResult` → `acceptResult` →
  `/pass/kuaishou/login/qr/callback` → `/account/login/api/verifyToken`
- 当前实现依赖 cookie jar 持久化;成功 cookie 含 `kuaishou.web.cp.api_st`、
  `kuaishou.web.cp.api_ph`、`passToken`。
- 2026-11-20 抽样:`start` 返回 `qrLoginToken`、`qrLoginSignature` 与 base64 PNG;
  未扫码状态下 20 秒后轮询返回「二维码已过期」,符合页面短时效行为。

### 微博

- `Troray/weibo-backup`:`src/auth/QRLogin.ts`,手动跟随重定向并复制跨域 cookie
- `netcccyun/weiboapi`:`includes/lib/WeiboLogin.php`,新版 `/sso/v2` 流程
- `axiaoxin-com/weibo`:`qr_login.go`,旧版 `alt` 换票
- 新版链路:`/sso/v2/qrcode/image` → `/sso/v2/qrcode/check` → `data.url` 换票;
  旧版链路则用 `alt` 调 `login.sina.com.cn/sso/login.php`。
- 关键坑:`http.Client` 自动跟跳会丢中间 `Set-Cookie`;必须手动逐跳合并。

### 小红书

- `ReaJason/xhs`:`example/login_qrcode.py`,需要外部签名函数
- `chinatsu1124/xhs-search-workflow-skill`:`xhs_auth.py`,纯请求 + 匿名 cookie bootstrap
- 链路:`/login/qrcode/create` → `/login/qrcode/status`(或 `/api/qrcode/userinfo`)
  → `/login/activate`。
- 仓库 issue 记录:确认后可能 HTTP 471/461 并返回 `Verifytype`/`Verifyuuid`;
  `xhs-cli` 已改为浏览器辅助并等待用户完成验证码。

### 抖音

- `3441293738/creatorhub`:`app/platforms/douyin/qrlogin.py`,依赖 `curl_cffi` 模拟 TLS
- `Laihiujin/Prism`:`app_new/platforms/douyin_http.py`,包含 `a_bogus`、SecSDK CSRF
- Prism 代码记录了 HTTP 会话缺少 verifycenter 信任时,确认交接可能返回 `2156`;
  因此浏览器路径仍是当前更稳选择。

### B站与豆瓣

- B站:`SocialSisterYi/bilibili-API-collect` 的二维码登录文档;项目已在
  `components/platform-bili/login.go` 实现。
- 豆瓣:`xijin285/MoviePilot-Plugins` 的 `douban_qr.py`、`tripodxu/film-sort` 的
  `worker/douban.ts`;项目已在 `components/platform-douban/login_http.go` 实现。

### 其他

- 知乎:`BAIGUANGMEI/zhihu-cli` 有纯 HTTP 实现;当前接口易触发环境验证。
- 网易云:`tianyupaipai-cmd/netease-music-mcp` 的 `src/qr-login.js` 说明 weapi 加密、
  801/802/803/800 状态与 `MUSIC_U`/`__csrf` 提取。

## 资源拦截与扫码

载具默认拦截图片、字体、媒体与统计域名,见 `components/service-browser/blocked.go`。
默认策略覆盖所有浏览器 target,扫码登录不需要额外开关;纯 HTTP 平台根本不进
载具,因此不参与这条策略。

- 抖音:先扫描 `#animate_qrcode_container` 内的 `data:image` 或 canvas,容器缺失才回退整页;
- 小红书:二维码通常是 `img.qrcode-img` 的 data URL,canvas 兜底;
- 微博/豆瓣:只读 `img.src`/接口返回的 URL,由宿主 HTTP 下载并转 data URL;
- B站:完全走 HTTP `generate → poll`,二维码内容由宿主页面本地编码,不加载图片。

实测(2026-11-20,真实 WebView2,每平台 blocked/open 各 3 轮交错执行,关闭
HTTP 缓存,取中位数;`ready` 为对应登录页出现二维码/可扫码信号的耗时):

微博、豆瓣当前优先纯 HTTP 取码;下表这两行只代表浏览器兜底登录页,不代表
实际登录链路。B站不经过载具,也不参与这组对比。

| 平台 | blocked ready | open ready | blocked load | open load | blocked 图片 | open 图片 |
|---|---:|---:|---:|---:|---:|---:|
| 微博 | 252ms | 251ms | 103ms | 98ms | 3 个 / 0KB | 3 个 / 0KB |
| 小红书 | 1977ms | 2026ms | 1843ms | 1887ms | 0 | 0 |
| 豆瓣 | 477ms | 472ms | 245ms | 240ms | 5 个 / 0KB | 4 个 / 11KB |
| 抖音 | 21402ms | 35089ms | 0ms | 0ms | 1 个 / 0KB | 1 个 / 12KB |

注:表中 `load=0` 表示该页在取样窗口内没有发出 `loadEventEnd`,不代表 0ms 加载完;
抖音按登录页出码信号计时,不按 `readyState complete`。这组数字是全新 profile、
关闭 HTTP 缓存后的冷启动基线,不能直接当作日常热路径耗时。

结论:

- 微博、小红书、豆瓣的差异都在网络波动范围内,图片拦截对出码没有可测的负面影响。
- 抖音的页面就绪波动极大(8.5–37.9s),blocked/open 样本重叠;当前数据不能证明
  图片拦截让抖音更快,也不能证明它变慢。用户日常热启动实测明显快于这组冷启动
  数字。抖音本身仍建议保留浏览器路径。
- 微博若整域拦截 `sinaimg.cn`,页面 JS 模块会被一起拦掉,二维码永远不出现;
  这次已将规则收窄为图片后缀和明确的图片路径。豆瓣的 `doubanio.com` 同理,
  不能整域拦。
- 图片请求计数来自 `PerformanceResourceTiming`;blocked 轮次仍会看到少量条目,
  这是被阻断请求的 timing 记录,不代表图片真正下载。字节数按 `transferSize`
  优先、`decodedBodySize` 兜底。

因此默认拦截图片不会破坏现有扫码取码;它只阻止页面解码展示图片。若未来引入
「必须读图片 naturalWidth/布局」的平台,应显式豁免该登录 target,而不是全局关掉。
新增无扩展名图床时,只能拦明确的图片路径,不能按整个静态域名拦。

### 登录页资源实测(2026-11-20)

用真实 WebView2 载具记录 Network 请求(一次性探针,未进入生产代码):

| 平台 | 请求数 | 总量 | 主要构成 |
|---|---:|---:|---|
| 小红书 `/login` | 124–125 | 3.1–3.4MB | Script 29 个 / 约 3.0MB |
| 抖音 `/login_page` | 41–52 | 0.7–1.0MB | Script 22 个 / 约 0.8MB |

小红书的大头是 `fe-static.xhscdn.com` 上的主包、`vendor`、`vendor-dynamic` 和
`redcaptcha`。`apm-fe.xiaohongshu.com`(监控上报)约 20 个请求、`t2.xiaohongshu.com`
(埋点)约 9 个请求,两者合计只有约 10KB;这些请求大多在二维码出现后才发出。
默认拦截之外再禁这两个域名,冷/热缓存下出码仍在约 1.1–1.9s,没有可测收益,
不建议加入默认规则。

抖音的大头是登录页异步分片(`518`、`630`、`334`、`741`、`774`)、验证中心脚本
和 `bdms` 安全脚本。`mon.zijieapi.com` 约 5 个请求、`vcs.zijieapi.com` 约 2 个
请求,合计约 6KB;额外阻断后冷启动仍为 29–49s 或超时,默认组为 30–49s,
没有改善。保留缓存时两者第二轮都约 0.9s,说明真正的冷启动瓶颈是静态资源
下载,不是监控请求。异步分片和安全/验证脚本不能拦,否则登录或扫码确认链路
会被破坏。

唯一体积较大的非核心资源是 `scan-code-guide-side.json`(约 84KB 的扫码视觉
引导);单独阻断后出码约 28.4s,默认约 28.9s,无实质收益,且可能影响页面引导,
不加入默认策略。

结论:现有默认策略(图片、字体、媒体、统计域名)已经覆盖了安全且确定的收益;
针对小红书/抖音再拆监控或埋点只减少少量请求,不会改善出码。更有效的方向是
复用 profile 缓存(热启动约 0.9s)或做登录页预热,而不是扩大 URL 黑名单。

## 抓码耗时与回归

2026-11-20 修复后实测(`LoginStart`,真实网络;浏览器项使用本机已热起来的
WebView2 环境,只作量级参考):

| 平台 | 路径 | 耗时 | 说明 |
|---|---|---:|---|
| 微博 | HTTP `image → check` | 0.3–0.4s | 直接取码;image/check 不需要 CSRF 预热 |
| 豆瓣 | HTTP `qrlogin_code` | 0.3–0.6s | `qrlogin_code` 自带 `bid`,无需先访登录页 |
| 小红书 | 浏览器 `/login` 独立页 | 2.3–2.6s | 直接出码,不需要点首页「登录」按钮 |
| 抖音 | 浏览器 `/login_page` | 12–49s,偶发超时 | 冷启动静态 JS 分片下载;热缓存第二轮约 0.9s |

B站走纯 HTTP `generate`,不进入载具,也不参与图片拦截/页面加载对比;不要把它
列进浏览器耗时表。

### 空白二维码根因

微博、豆瓣的二维码此前有两种“空白”表现,根因不同:

1. **空白框**:`fetchImage` 下载失败时回退成远程图片 URL,而载具默认拦截图片;
   前端拿到的 `<img>` 永远不会加载,看起来就是空白。现在失败返回空串,由上层
   明确报错,不再回退远程 URL。
2. **合法 PNG 外壳但只有 1 种颜色**:只出现在早期实测样本里,没有稳定复现。
   当时的 `GetBytes` 曾用 `[]byte(res.Body)` 取字节,后来回归测试
   `TestGetBytesPreservesBinary` 证明 Go 的 `string` 可无损承载任意字节,
   这一步本身不会损坏 PNG;因此不能把“字符串中转”当作已证实根因。

当前实现仍然让 `platform.Response` 保留原始 `Raw []byte`,`GetBytes` 直接返回
原始字节;这是一条更直接、可测试的边界,避免以后在传输层出现任何转码。这一轮
改完后实测微博二维码为 180×180、66 色,豆瓣为 195×195、2 色,均正常;但样本
不足以单独证明是哪一处改动消除了 1 色 PNG。

`fetchImage` 现在还会校验 `Content-Type` 与 PNG/JPEG/GIF/WebP 魔数;校验失败或
下载失败都返回空串,不会把错误页/空图编码成 data URL 推给前端。

### 预热到底在哪一步

“预热”指 `httpStart` 里在取码之前额外访问登录页,目的是拿 cookie/CSRF,不是
载具预热,也不是提前打开浏览器 target。当前只有需要它的链路才保留:

```text
微博(已去掉):LoginStart → httpStart
  旧:GET passport 登录页 → GET /sso/v2/qrcode/image → GET 图片
  现:GET /sso/v2/qrcode/image → GET 图片

豆瓣(已去掉):LoginStart → httpStart
  旧:GET accounts.douban.com/passport/login → GET /qrlogin_code
  现:GET /qrlogin_code(自己下发 bid)
```

微博的 `X-CSRF-TOKEN` 只用于 `/sso/v2/web/config` 和密码登录,扫码取码/轮询都不
依赖它;因此能省掉第一步,实测 `LoginStart` 从包含预热降到约 0.3–0.4s。豆瓣的
`qrlogin_code` 自己下发 `bid`,`LoginStart` 约 0.3–0.6s。

### 抓码链路现状

| 平台 | 取码 | 轮询/成功判定 |
|---|---|---|
| 微博 | HTTP `/sso/v2/qrcode/image`,图片由宿主下载转 data URL | `/sso/v2/qrcode/check`,成功后换 ticket,取 `SUB` 等 cookie |
| 豆瓣 | HTTP `/j/mobile/login/qrlogin_code`,图片由宿主下载 | `/j/mobile/login/qrlogin_status`,`login` 后补访成功页/首页取 `dbcl2` |
| B站 | HTTP `generate`,返回二维码内容字符串,页面本地渲染 | HTTP `poll`,成功时从 `Set-Cookie` 取 `SESSDATA` |
| 小红书 | 浏览器直达 `/login`,读 `img.qrcode-img` 的 data URL,canvas 兜底 | 读载具 cookie 的 `user_id` 相对基线变化,或页面 SSR/DOM 翻转为已登录 |
| 抖音 | 浏览器 `/login_page`,读页面内 `data:image/png`,容器 `#animate_qrcode_container` 优先 | 载具 cookie 出现 `sessionid`(有 `ttwid` 只是游客) |

补充约束:

- 微博轮询间隔保持 2s,避免无意义高频请求;`qrcode/check` 未扫码返回 `50114001`,
  属正常 pending。`X-CSRF-TOKEN` 不参与扫码取码/轮询。
- 图片拦截对以上路径无影响:微博/豆瓣由宿主 HTTP 取图,小红书/抖音读 data URL
  或 canvas。不要整域拦 `sinaimg.cn`、`doubanio.com`,它们同时承载 JS/CSS。
- 小红书保留 3 次重试、每次最多 10s;轮询期间也检查已登录态。
- 抖音保留 `data:image/png` 与 PNG 头尺寸 ≥200×200 过滤;选择器只影响扫描成本,
  不影响站点出码时间。
- 小红书/抖音的脚本导航在读取二维码前会等 `performance.timeOrigin` 变化;刷新
  时不会再返回导航前的旧码。过渡期的 evaluate 异常仍按「未就绪」处理。

抖音本轮还确认了:页面在 5–6s 才挂出二维码,且多数轮次 `readyState` 长时间停在
`interactive`;4 秒的 `load` 等待会直接吞掉出码机会,所以用 `location.assign` +
轮询,不走 `Navigate` 等 `complete`。

### 抖音为什么曾扫所有图片

不是业务上想遍历全页,而是二维码由页面内脚本即时生成成 `data:image/png`,没有稳定
的图床 URL 可用。旧实现只能扫描所有 `img[src^="data:image"]`,再按 PNG 头尺寸
过滤掉 logo/占位图。实测二维码结构为:

```text
img.RhjdbXj8 → div.XI37I0dP → div#animate_qrcode_container → div#douyin_login_comp_scan_code
```

现在优先只扫 `#animate_qrcode_container`,容器不存在才回退整页;仍保留
`data:image/png` 与 PNG 头尺寸 ≥200×200 过滤,因为容器内也可能混入非二维码图片,
站点类名会变。

### 小红书为什么能省掉“登录”按钮

小红书有独立登录页 `https://www.xiaohongshu.com/login`,访问后直接弹扫码面板。
旧实现先加载首页,轮询并点击「登录」按钮,再读二维码;首页脚本重,且按钮结构
随版本变化。改为 `location.assign('/login')` 后直接轮询二维码,实测约 1.3s 出码,
`LoginStart` 约 2.3–2.6s。已登录用户访问 `/login` 时也要能识别并返回 `already`,
所以轮询期间仍检查页面已登录态。

### 抖音冷启动为什么慢

同一 profile 保留 HTTP 缓存时,第二轮出码约 0.9s;第一轮(冷启动)约 12–49s。
CDP 时序显示 `get_qrcode` 往往在 48s 才发出,而 `518/630/498/741/774` 等异步
分片从 17.8s 持续下载到 48s,响应头为 `cache-control: max-age=2592000`。
因此瓶颈是**首次拉取静态 JS 分片**,不是二维码选择器,也不是监控请求。

可选方向:复用载具 profile 缓存(生产默认已保留),或在打开登录页之前做一次
轻量预热;不要通过扩大 URL 黑名单来解决。

### 刷新二维码为什么慢

刷新按钮没有单独的刷新接口,而是再次调用 `LoginStart`。此前 `LoginStart` 在
浏览器路径上固定用 `Cookies` 读已有登录态;这个调用会重新 `Acquire` 同一
profile,而旧二维码的登录 target 仍被 pin 住,于是刷新请求一直卡到旧会话释放。
小红书和抖音都在这里付出了不必要的等待。

现在会按 `login:<平台>` 走 `CookiesAs`,命中已 pin 的实例;同时脚本导航会先记
`performance.timeOrigin`,只在文档真正切换后读取二维码,避免把刷新前的旧码
当成新码返回。旧轮询的收尾也改为只删除自己那个会话,不会误关心会话。

2026-11-20 实测(同一 `service-credentials.Service` 连续两次 `LoginStart`,不先关闭旧
target,临时 profile):

| 平台 | 首次 | 第二次(刷新) |
|---|---:|---:|
| 小红书 | 2.077s | 1.219s |
| 抖音 | 2.318s(该轮缓存已热;冷启动另见上节) | 910ms |

第二次已经不是“等旧 pin”,抖音的数字与热缓存出码一致;首次仍受冷启动资源下载
影响。

## URL 拦截与 WebView 图片渲染开关

当前实现按 URL 在 CDP 网络层拦截,没有关闭 WebView 的图片渲染。

- 运行时可控:登录 target 若确实需要图片解码,可单独给它换拦截表;全局开关做不到
  这种按 target 的细粒度。
- 省的是下载、解码和内存,不只是绘制。关渲染仍可能把图取下来,对抓取页没有收益。
- 不改变 DOM 结构。按 URL 拦会让 `img` 保留节点但加载失败,依赖 `img.src`/DOM
  的取码逻辑仍能工作;关渲染不会移除节点,但页面内脚本仍可能读到已加载的图片,
  排查时不容易区分"没请求"和"没绘制"。
- 现有平台没有依赖 `naturalWidth` 或图片布局的取码路径,因此不需要为了渲染兼容
  放弃网络层拦截。

如果未来确实遇到必须加载图片的登录页,优先给该 target 单独豁免,而不是关闭全局
图片拦截;只有需要"保留请求但不绘制"的截图/视觉场景,才考虑渲染开关。
