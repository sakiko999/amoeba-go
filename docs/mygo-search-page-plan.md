# mygo-host 聚合搜索子页 阶段 1 方案

> 2026-11-17。调研 workflow(4 维度并行只读:宿主 UI 接入点 / 各平台搜索能力 / amoeba 搜索参考 / 契约设计)汇总裁决。
> 需求:新增聚合搜索子页,阶段 1 可搜任意平台直播间,结果标注平台/主播/直播间信息,参考 amoeba 原有搜索。
> 关联:apps/mygo-host(Go 宿主)、packages/platform-*、packages/tab-*(页面组件)。

**推荐架构:页面组件 `apps/mygo-host/components/page-search` + 宿主 `search` 服务 fan-out + 各平台统一 `<ns>.search` op。**
一句话理由:gateway 必须保持无域逻辑(注册/分发),页面应最小权限;并发、超时、结构化部分失败、
取消与流式回传只有宿主服务层能统一做好。

```text
apps/mygo-host/components/page-search(页面组件,无 preload/main)
  window.mygo.call("Search.Sources", scope)
  window.mygo.call("Search.Query", params, mygo.channel)
        │
        ▼
apps/mygo-host/search(宿主 Go 服务,BindAs("Search"))
  Sources:按 tag 发现平台;Query:goroutine 并发 fan-out + 8s/平台超时
  + 错误分类 + 分组流式回传 + context 取消
        │  gateway.Call("<ns>.search", {keyword,scope,page,pageSize})
        ▼
api-gateway(只注册/分发,不含域逻辑)
  ├─ bili.search    live=searchBiliRooms      channel=searchBiliAnchors
  ├─ douyu.search   live=searchRooms          channel=searchAnchors
  ├─ huya.search    live=searchRooms          channel=searchAnchors
  ├─ youtube.search live=searchYouTubeLive    channel=searchYouTubeChannels
  └─ twitch.search                           channel=searchTwitchChannels
```

- 增量回传用 mygo 原生 Channel:`Query(ctx, params, groups *mygo.Channel[PlatformGroup])`;
  页面 `window.mygo.channel(cb)` 创建,平台一完成就推一条分组,不轮询。
- 页面不绑定 Gateway;阶段 1 只暴露窄接口 `Search.Sources` / `Search.Query`。
- UI 用 React 19 + Tailwind 4 + @demo/ui(同 home/media-manager);双 scope、平台 chips、
  分组流式、逐平台错误/空态/取消的交互量不适合 vanilla 先行。

## 1. 宿主 UI 接入点(调研结论)

- 新增页面**不需要改 chrome、tabs manager、menu**:页面组件目录放进 `packages/`、在
  `scripts/pack-components.ts:19-29` 登记、投影到 `apps/desktop/out/pages` 即自动出现在
  「+」菜单(宿主启动扫描 `apps/mygo-host/main.go:40-42`、菜单生成 `tabs/manager.go:233-244`)。
- registry 只在启动时 Scan,新增页面后必须重启 `mygo:dev`。
- 页面 → 宿主桥:mygo 先注入 `window.mygo`,`tabs/pagebridge.go:10-32` 包装成 `window.tab`;
  现有唯一 `BindAs` 是 Tabs(`main.go:75`)。**缺口:`services.Activate` 返回的 Gateway 在
  `main.go:61` 被丢弃**,页面无法调任何 op——本方案以 `Search` 窄服务补,不透出 Gateway。
- 样板:最小包 `packages/tab-clock`(无 preload);React 形态 `packages/home`
  (`electron.vite.config.ts:21-44` vendor external + import map、`scripts/pack.ts` 的
  `PAGE_VENDOR_EXTERNAL=1`),照抄但**删掉 preload 段**。
- `component.json` 校验规则在 `apps/mygo-host/registry/manifest.go:73-102`;**不要写
  main/preload**(v1 解析但忽略,写了多一条存在性校验);`inject` 只校验未消费,不写。
- mygo 绑定约束:`BindAs` 校验全部导出方法参数/返回值 JSON 可编码
  (mygo@0.9.0 `ipc.go:124-157`);**不要用 `json.RawMessage` 当参数**(`[]byte` 是 base64
  字符串),用 `any`;返回 `any` 合法(tsgen → unknown)。

## 2. 阶段 1 范围

| scope | 平台 | 底层能力 | 阶段 1 |
| --- | --- | --- | --- |
| `live`(搜直播间) | bili | `searchBiliRooms` 已实现未接渠道(platform-bili/discover_search.go:16) | 接入 |
| `live` | douyu | `searchRooms` 已实现未接渠道(platform-douyu/discover.go:121) | 接入 |
| `live` | huya | `searchRooms` 已实现未接渠道(platform-huya/discover.go:113) | 接入 |
| `live` | youtube | `searchYouTubeLive` 已实现无调用方(platform-youtube/search.go:75) | 接入 |
| `live` | twitch | 无直播搜索;需 searchFor→streams 两步 enrich(platform-twitch/client.go:181) | 阶段 2 |
| `channel`(搜主播) | bili/douyu/huya/youtube/twitch | 全部现成(searchBiliAnchors/searchAnchors×2/searchYouTubeChannels/searchTwitchChannels) | 接入 |
| — | apple 播客、tmdb/bangumi 元数据 | 已有各自 search(垂直域,豆瓣无搜索) | 不做 |

裁决要点:
- **live 优先搜房间**:房间搜索的 Item 已带 Platform/RoomID/LiveStatus/Online;上游
  tauri-rss 删除「关键词搜房间渠道」是订阅场景决策,不照搬。
- bili 的 `channel` 映射主播(`searchBiliAnchors`)而非 UP 主(`bili:user` 留给未来 scope)。
- 搜索**不做成 channel**(渠道带订阅语义);现有 `bili:anchor` 等渠道保持不动。

## 3. 结果契约

**复用 `platform.Item`(platform/feed.go:88),阶段 1 不新增字段、不改现有平台映射。**
平台身份用 group.namespace(不依赖 channel 结果里可能为空的 `Item.Platform`)。

各平台字段填充情况:

| 字段 | live 房间 | channel 主播/频道 |
| --- | --- | --- |
| `title` | 房间标题 | 主播名/频道名 |
| `author.name` | bili/douyu/huya/youtube 有 | 部分(youtube/twitch 频道无) |
| `summary` | 部分 | bili UP/YouTube/Twitch 频道有(仅展示,**禁止解析数字**) |
| `thumbnail` | 封面/头像 | twitch 频道无图 → 平台徽标兜底 |
| `url` | 全部 | 全部(系统浏览器打开) |
| `platform`/`roomId`/`liveStatus`/`online` | 有(见风险②③) | 无/部分 |

### 3.1 平台侧 `<ns>.search` op(新增 platform/searchops.go)

```go
type SearchScope string // "live" | "channel"

type SearchParams struct {
    Keyword  string      `json:"keyword"`
    Scope    SearchScope `json:"scope"`
    Page     int         `json:"page,omitempty"`     // 从 1 起,默认 1
    PageSize int         `json:"pageSize,omitempty"` // 默认 20,平台钳制
}

type SearchSpec struct {
    Scopes []SearchScope
    Search func(ctx context.Context, p SearchParams) (FetchPage, error)
}

func SearchOps(spec SearchSpec) Ops        // 校验 keyword/scope/默认值,保证 Items 非 nil
func SearchTags(scopes ...SearchScope) []string // -> ["search.live", "search.channel"]
func MergeOps(base, extra Ops) Ops
```

返回复用 `FetchPage`(platform/channel.go:27);阶段 1 不填 total/cursor/hasMore,
映射不填 `Item.Raw`。

### 3.2 宿主 Search 服务(新增 apps/mygo-host/search/service.go)

```go
type PlatformGroup struct {
    Namespace string          `json:"namespace"`
    Label     string          `json:"label,omitempty"`
    Items     []platform.Item `json:"items"`
    Error     *GroupError     `json:"error,omitempty"` // {op, code, message}
    ElapsedMS int64           `json:"elapsedMs"`
}

type Result struct {
    Keyword   string
    Scope     platform.SearchScope
    Groups    []PlatformGroup // 按 Sources 顺序,权威快照
    ElapsedMS int64
}
```

- `Sources(scope)`:gateway.List() 过滤「platform.data + search.<scope> + 含 search op」。
- `Query`:校验参数;每平台一个 goroutine,`context.WithTimeout(8s)`;**单平台失败是
  group.error 数据,不使 Query 失败**(顶层只在参数非法/父取消时 error)。
- 每完成一个平台 `groups.Send`;`Result.Groups` 按 Sources 顺序。
- 错误分类:DeadlineExceeded→timeout / Canceled→canceled / PlatformOpError→platform / 其余 internal。
- 取消:关 tab→mygo 取消 context;新搜索→页面 `channel.close()`→旧调用取消。
- 不做跨平台去重、全局混排、全局限流(阶段 1 请求量小,取消+超时兜底)。

## 4. 实现步骤(文件级,依赖序)

1. `platform/searchops.go` + `searchops_test.go`(契约与校验)。
2. 五平台 `search_ops.go` + 各 platform.go Tags/MergeOps(bili/douyu/huya 双 scope,
   youtube 双 scope(复用 `youtubeLiveHitToLive`、抽 `channelHitsToItems`),
   twitch 仅 channel)。
3. `services/services_test.go` 断言 tags(四平台双 scope、twitch 仅 channel、apple/元数据无)。
4. `search/service.go` + `service_test.go`(发现/过滤/错误/超时/流式/顺序;测试直调 run)。
5. `main.go`:接住 `services.Activate` 返回(当前 main.go:61 丢弃),`BindAs("Search", ...)`。
6. 顺带根因修复:`tabs/pagebridge.go` 初始主题写死 dark,接 `m.theme.Theme()`。
7. `apps/mygo-host/components/page-search/` 新包:component.json(id=search,无 main/preload/inject)、
   package.json、electron.vite.config.ts(照 home 删 preload)、scripts/pack.ts。
8. 页面源码:index.html(CSP `img-src https:` + `referrerPolicy="no-referrer"`)、
   api.ts(镜像 DTO,不改 @host/sdk)、App/SearchBar/PlatformChips/ResultGroup/ResultItem。
9. `scripts/pack-components.ts` 登记 + 120KB 体积门禁清单。
10. 验证:`bun run pages:prepare` → `sync-pages.ts` → `mygo:dev`;`go test ./...`、
    typecheck、lint。
11. 手动验收:`+` 菜单出「搜索」;live 搜「英雄联盟」四组增量出现;channel 搜
    「shroud」出 twitch;单平台 chip;取消;断网/超时分组错误;主题首帧。

## 5. 交互设计(借 amoeba/tauri-rss)

- **一处输入、多平台并发**,平台 chips「全部/单平台」;Enter 触发但避开
  `isComposing` 中文组合态;空关键词禁用;不做输入即搜。
- **边到边填**:流式分组,不阻塞最慢平台。
- **单源失败不影响全局**:每平台一个源,简化为「该组带 error,其余照常」。
- **候选行**:缩略图(无图平台徽标)+ 标题 + 次行(author/统计)+ meta chips(在线/房号)+ 平台徽标。
- **请求世代守卫**:新搜索先 close 旧 channel(自动取消)+ 页面 gen 计数。
- **空态三分**:未搜索 / 0 条 / 全部平台失败;搜索按钮不清空已有结果。
- 取数源不硬编码:启动/切 scope 调 `Search.Sources(scope)` 渲染 chips(不猜 sourceInfoTpl 参数名)。
- 不照搬:XML 往返、订阅落地 targets/sticky 操作条、本地库内搜索(上游已删)。
- 结果卡片 `<a target="_blank">`,mygo 默认系统浏览器打开;应用内播放属阶段 2。

## 6. 工作量

| 块 | 人日 |
| --- | --- |
| 平台 op 契约 + 五平台接入 + 测试 | 2–2.5 |
| 宿主 Search 服务 + 绑定 + 主题修复 | 1.75 |
| 页面脚手架 + UI 交互 | 2–2.5 |
| 联调与真机验收(5 平台真实搜索与映射修正) | 1–2 |
| 回归收尾 | 0.5 |
| **合计** | **7–9** |

砍 channel scope 减约 1 人日;twitch 再推迟减 0.25;真实网络调优(cookie/反爬/字段修正)
不可控,可能额外 1–3 人日。

## 7. 风险与待确认

1. **匿名搜索真机未验证**(最高):bili 依赖匿名 buvid 引导(nav.go:105)、youtube 依赖
   visitorData、douyu/huya 依赖特定 UA/referer;仓库内只有离线 fixture。缓解:分组错误
   隔离 + 真机逐平台验收,必要时降级该平台。
2. **huya 房间搜索 liveStatus 恒 live**(map.go:156,与 TS 一致),可能误报;先保持移植
   一致,真机确认后改 gameLiveOn 或 unknown。
3. **youtube 直播搜索可能混入普通视频**(固定标 live);真机确认,必要时解析 badge。
4. **mygo 0.x `*mygo.Channel[T]` 流式绑定**需上线前 go run 确认;回退:非流式 Query
   返回 Result(页面 await),架构不变。
5. Item 语义随平台漂移(kind/platform/title/summary);UI 以 group.namespace 为准。
6. 远程缩略图 CSP 放行 https + no-referrer + onError 徽标兜底;更严隐私再改 Go 侧转 data URL。
7. ACL 未落地(Search 全局绑定,v1 内置页面可信可接受);外部页面安装前把 inject 升格 ACL。
8. 新页面需重启宿主(registry 启动时才 Scan)。

## 8. 架构冲突裁决记录(备查)

| 冲突 | 裁决 |
| --- | --- |
| 页面直连 Gateway vs 宿主 Search 服务 fan-out | 后者(结构化错误/超时/取消/限流只在宿主层做得好;直连快 ~1.5 人日但每个页面重复) |
| 复用 channel fetch vs 新增 search op | 新增 op(fetch 参数名不统一且搜索不该带订阅语义) |
| 搜房间 vs 搜主播 | live 优先房间,channel 同页第二 tab |
| twitch 参与 live? | 否,阶段 2 enrich |
| Item vs 新 SearchHit | 复用 Item,需要更严契约再加包装层 |
| 分页 | 只取首屏 20 条,契约保留分页字段 |
| 错误策略 | 错误作为分组数据,顶层只在参数非法/取消时 error |
| 增量渲染 | mygo Channel 流式,非轮询 |
