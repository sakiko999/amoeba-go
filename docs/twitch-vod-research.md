# Twitch 直播回放(VOD)接入调研

> 2026-11-16。调研 agent 实测(匿名,无 OAuth,只读 GQL/usher GET/POST)。
> 实测 VOD:2929172303(shroud 场次);实测主播:shroud(totalCount 1607)。
> 关联:packages/platform-twitch(TS 现行)、apps/mygo-host/components/platform-twitch(Go)。

**结论:有条件能接。** VOD 列表、详情、播放 token、usher master、弹幕评论分页均
匿名实测通过,Go 侧接入规模与 douyu replay 同量级(2–3.5 人日,不含播放器)。
两个硬条件:

1. **播放端是真正约束**:仓库内没有任何播放器(Go 侧只解析到 `Stream[]`);
   且 Twitch **VOD** 的 variant playlist 与分片**无 ACAO 头**(直播有
   `ACAO:*`),若播放器用 hls.js 必跨域失败——需宿主代理或 WebView2 原生 HLS。
2. **无登录边界**:公开 VOD 匿名可用;sub-only/私密/已删除 VOD 需 OAuth
   (宿主 Credentials 只存 cookie,不覆盖,本期不做)。

## 1. 现状盘点

渠道 Go/TS 一一对应,均 5 个(`twitch:live/hot/category/categories/user:search`),
**无任何 VOD 渠道**。直播流链路:GQL `PlaybackAccessToken` persisted query →
usher master → Go 侧服务端拉取解析 master(usher 无 ACAO,必须 Go 拉)→
只把 variant URL 交给播放器。直播弹幕 Go/TS 均未实现(IRC over WS 二期)。

- Go 取流:`components/platform-twitch/playback.go:20`(persisted hash)、
  `:72-101`(token)、`:126-155`(master URL + 服务端拉取解析);
  `master.go:33-107` 解析 `EXT-X-STREAM-INF`。
- TS:`packages/platform-twitch/src/lib/live.ts:23/:45-97/:100-120/:171-221`。
- Client-ID:网页 `kimne78kx3ncx6brgo4mv6wki5h1ko`,移动端
  `kd1unb4b3q4t58fwlpcbzcbnm76a8fp`(`client.go:17-20`)。
- **仓库内无播放器**:`tabs/manager.go:128-148` 把 tab 指向页面包 EntryURL,
  页面组件无 `<video>`、无 hls.js/dash.js 依赖。本机 WebView2 157 的
  Edge `canPlayType("application/vnd.apple.mpegurl")` 返回 `maybe`
  (Chromium 142+ 原生 HLS),未实际验收。

## 2. 回放数据源

### VOD 列表(GQL,注意不是 channel.videos)

```graphql
query {
  user(login: "shroud") {
    videos(first: 30, type: ARCHIVE, sort: TIME) {
      totalCount
      pageInfo { hasNextPage }
      edges { cursor node { id title lengthSeconds createdAt publishedAt
        viewCount status broadcastType
        previewThumbnailURL(width: 640, height: 360)
        owner { login displayName } game { name } } }
    }
  }
}
```

- `type`:ARCHIVE(过往直播)/ HIGHLIGHT(高光)/ UPLOAD;`sort`:TIME/VIEWS;
  `first` 实测 1–100。
- **翻页 `after` 必须用移动端 Client-ID**:网页 ID 带 `after` 返回
  `data.user.videos = null`(integrity 校验失败);首页两 ID 均可。
- `edges[].cursor` 同页可能全部相同,取最后一个;`pageInfo` 只有
  `hasNextPage` 无 `endCursor`。

### VOD 详情

```graphql
query {
  video(id: "2929172303") {
    id title description createdAt publishedAt recordedAt updatedAt
    broadcastType lengthSeconds status viewCount
    previewThumbnailURL(width: 640, height: 360) seekPreviewsURL
    game { id name slug } owner { id login displayName }
  }
}
```

匿名可用。`status`:`RECORDED` / `RECORDING`(正在直播、仍在增长的 VOD,
`lengthSeconds` 持续增长——建议过滤或标记)。

### 播放 token 与 usher(复用现有 persisted query,变量切 VOD)

```json
{
  "operationName": "PlaybackAccessToken",
  "variables": { "isLive": false, "login": "", "isVod": true,
    "vodID": "2929172303", "platform": "web", "playerType": "site" },
  "extensions": { "persistedQuery": { "version": 1,
    "sha256Hash": "ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9" } }
}
```

- 返回 `videoPlaybackAccessToken { value signature }`;匿名成功,
  `privileged = false`。**VOD token 有效期约 20 小时**(直播约 20 分钟)。
- 进行中的 VOD token 带 `is_dvr = true`。
- usher VOD master(实测 200):

```http
GET https://usher.ttvnw.net/vod/v2/{vodID}.m3u8
  ?sig={signature}&token={value}&allow_source=true&allow_audio_only=true
  &platform=web&player_backend=mediaplayer
  &playlist_include_framerate=true&supported_codecs=h264
```

推荐与现有直播代码一致的 `sig/token` + `/vod/v2/` 形态(twitch-dl 的
`/vod/{id}?nauth=` 旧形态也能用)。`include_unavailable=true` 会多出隐藏档位,
默认不加。

## 3. 播放链路差异(直播 vs VOD)

| 项 | 直播 | VOD |
| --- | --- | --- |
| master | `usher.ttvnw.net/api/v2/channel/hls/<login>.m3u8` | `usher.ttvnw.net/vod/v2/<id>.m3u8` |
| master ACAO | 无 | 无 |
| variant/分片 CORS | `ACAO: *`(hls.js 可播) | **无 ACAO(hls.js 必败)** |
| playlist | `index-dvr.m3u8` 滚动窗口,无 ENDLIST | 完成后带 `ENDLIST` + `EXT-X-MAP` |
| 进行中 | 直播流 | `RECORDING` + `is_dvr=true`,EVENT 无 ENDLIST 持续增长 |
| 广告 | SSAI 拼接 | 抽样 3 VOD + 1 highlight 无广告分片;highlight 的 discontinuity 是剪辑边界,不是广告 |
| 静音 | 不适用 | 服务端静音(`index-muted-*`);`-unmuted` 路径 403,**不要改写 URL** |

**CORS 方案选型**(播放器侧):宿主代理(Go 拉 VOD playlist/分片加 CORS +
Range,约 2–4 人日)/ WebView2 原生 HLS(`<video src=.m3u8>`,本机 157
`canPlayType` maybe,未验证播放,可行则约 0.5–1 人日验证)。直播不受影响——
别因"直播能播"误推 VOD 也能播。

## 4. VOD 弹幕(videoComments)

persisted query `VideoCommentsByOffsetOrCursor`,hash:

```text
b70a3591ff0f4e0313d126c6a1502d79a1c02baebb288227c582044aa76adf6a
```

- 首页按偏移:`variables: { videoID, contentOffsetSeconds: 0 }`;
  后续页用 `cursor`(上一页最后 edge 的 cursor)。
- **cursor 分页必须移动端 Client-ID**(网页 ID 返回
  `IntegrityCheckFailed` 且 `comments = null`);偏移分页匿名可用,可作
  cursor 失效降级(需按评论 ID 去重)。
- 每页约 59 条(`first` 变量不生效);实测 6 页 353 条无重复,偏移 28→2969s
  递增。同一页所有 edge 的 cursor 可能相同,取最后一个。
- `VideoComments`(hash `be06407e…`)只返回元数据和 badges,**不含评论**,别用错。

字段映射(与 douyu `replay_danmaku.go` 同构):

- `contentOffsetSeconds` × 1000 → `TimeMs`(相对 VOD 起点,无需 createdAt 换算)。
- `message.fragments[].text` 拼接 → Text(fragments 可能含 emote,空文本跳过);
  `commenter.displayName` → User(**commenter 可能为 null**);
  `message.userColor` → Color;Mode 省略。
- `comments` 可能为 null(聊天关闭/无权限)。
- 分页成本:长 VOD 评论上万,10k 条 ≈ 170 请求;经验单 IP ~10k 条后可能限流
  (GQL ~120 RPM)。照 douyu 做「订阅即串行拉全量 + 退避 + 可取消」,
  59 条/页与 10k 上限写入已知限制;要实时弹幕再扩 `DanmakuInfo` 带播放位置。
- 直播聊天是 IRC 实时流,不是评论 API;两者不要混用。

## 5. 契约要点

- 新渠道 `twitch:vod`:kind=video;**游标翻页(after cursor)不能用
  `MakeChannel` 的数值 `Pageable`**(`platform/channel.go:62-74`),需自定义
  DataSource 实现 `PageableSource` 的 opaque cursor。
- 路由:`twitch.tv/videos/<id>` → kind=video;**注意 `feedops.go:216-222` 与
  TS `extension.ts:244-246` 的 id-only `play` 默认 kind 会从 live 变 video**,
  现有 `resolve_test.go:98-112` 的 `{"id":"shroud"}` 直播测试需显式
  `kind:"live"`(douyu replay 已有同类 caveat)。
- `GetDanmaku` 分派:`opts.Kind == "vod"` 走评论分页,默认 live(与 douyu 同)。
- sub-only usher 403 body 含 `vod_manifest_restricted` /
  `unauthorized_entitlements`;已删除/过期 VOD(token null 或 usher 404)
  应映射成明确的 NotFound 语义。默认保留期约 60 天。
- token/URL 含 sig,保持现有「不进错误文案」纪律(`playback.go:140-155`)。

## 6. 风险与未验证项

- GQL integrity/Client-ID(中高):网页 ID 已在 `videos.after` 与评论 cursor
  触发失败,当前靠移动端 ID 绕过;若 Twitch 收紧,需 integrity token(KPSDK)
  或降级为首页 + 偏移分页。
- 播放器缺失(高,前置问题):先定播放器方案,否则只能完成到 gateway `Stream[]`。
- 高光 VOD 的评论偏移是否与剪辑后时间轴一致未验证,不能假设从 0 对齐。
- fMP4(`EXT-X-MAP`)与相对分片路径:master 解析假定 variant URL 绝对地址,
  实测成立,应加相对路径兜底。
- TS↔Go 漂移(中):TS `playback.ts:7-34` 明确把 `/videos/` 留给后续
  (`tests/platform.test.ts:36` 断言不命中);Go-only 会分叉,同步 TS 另 +1.5–2 人日。
  `PlaybackAccessToken` hash 在 `lib/live.ts:23` 与 `playback.go:20` 已重复,
  新增评论 hash 应集中管理。
- 未验证:WebView2 原生 HLS 实播、sub-only 聊天(通常返回空)、限流真实阈值、
  `RECORDING` VOD 播放体验、4K 档位。

## 7. 工作量

| 项 | 估 | 说明 |
| --- | --- | --- |
| `twitch:vod` 渠道 + GQL 列表 + 游标翻页 + mapping | 0.5–1d | 自定义 DataSource opaque cursor |
| VOD 详情/路由 + ResolveVideoPlay | 0.5d | 复用 fetchPlaybackToken,usher 参数化 `/vod/v2/` |
| 评论分页 + 映射 + 后台流 + 取消 | 0.5–1d | 参考 `replay_danmaku.go:60-101` |
| httptest fixture + 真实烟测 | 0.5–1d | 列表/详情/token/usher/comments + 真实 VOD |
| **Go 合计** | **2–3.5 人日** | 与 douyu replay 同量级,评论更重 |
| TS 侧同步(可选) | +1.5–2d | 不做则记录 Go-only 分叉 |
| 播放器/CORS(hls.js 路径) | +2–4d | WebView2 原生 HLS 可行则 ≈0.5–1d 验证 |
| sub-only/OAuth(可选) | +1–2d | token 存取 + 设置界面 |

## 8. 推荐落地路径

1. Go-only 后端 MVP:`twitch:vod` 渠道 + `/videos/<id>` 路由 +
   ResolveVideoPlay + GetDanmaku(vod),补 httptest + 真实烟测。
2. 并行验证 WebView2 157 原生 `<video>` 能否直播 Twitch VOD variant
   (无 ACAO);可行则 VOD/直播共用原生链路,否则再评估宿主代理。
3. 暂不做 sub-only;错误文案明确「需要登录/订阅」。
4. 评论先「全量串行 + 取消 + 退避」,限制入文档。

## 9. 复现(只读)

```bash
# Client-ID: 移动端 kd1unb4b3q4t58fwlpcbzcbnm76a8fp(翻页/评论 cursor 必须)
curl -s -H "Client-ID: kd1unb4b3q4t58fwlpcbzcbnm76a8fp" \
  -H "content-type: application/json" -d '{"query":"{ user(login: \"shroud\") { videos(first: 30, type: ARCHIVE, sort: TIME) { totalCount pageInfo { hasNextPage } edges { cursor node { id title lengthSeconds status previewThumbnailURL(width: 640, height: 360) owner { login displayName } game { name } } } } } }"}' \
  https://gql.twitch.tv/gql
# 详情:video(id:) 同端点;token:PlaybackAccessToken persisted query(hash 见 §2)
# usher:GET https://usher.ttvnw.net/vod/v2/{id}.m3u8?sig=..&token=..
# 评论:VideoCommentsByOffsetOrCursor persisted query(hash 见 §4),
#       首页 {videoID, contentOffsetSeconds:0},后续 {videoID, cursor}
```
