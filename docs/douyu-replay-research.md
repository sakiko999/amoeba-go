# 斗鱼直播回放接入调研

> 2026-11-16 实测(只读 GET/POST,无 cookie;房间 71415、24422、9999)。协议现状
> 由 `components/platform-douyu` 的回放实现承载;本文保留端点语义与字段坑作回归参考。

**结论:有条件能接。** 回放列表、VOD 弹幕、详情页均可匿名访问,回放流是带短时
token 的 HLS VOD;唯一硬条件是取流要复用现有 crptext 签名链(依赖 JS 执行面,
与直播相同,不是新增依赖)。

## 1. 数据面

### 房间号 → up_id(回放按主播组织,不按房间)

```http
GET https://www.douyu.com/betard/{roomId}
```

`room` 对象含 `up_id`(哈希形如 `5JjALNBWgwXr`)、`owner_uid`(数字)。
**`authorShowVideoList` 只认哈希 up_id**:实测传 room_id/uid/数字 up_id 全部
`error:0, count:0` 静默为空。房间页 SSR 的 `$ROOM` 里也有 `up_id`,可作 betard
风控(返回 HTML)时的兜底。

### 回放列表

```http
GET https://v.douyu.com/wgapi/vod/center/authorShowVideoList?up_id={up_id}&page=1&limit=20
```

- `limit` 服务端上限 20(传 50/100 也只回 20);`page` 从 1 起;历史可回溯到 2017 年。
- `data.count` 是**场次总数**(非当前页条数);`data.list[]` 按场次组织。
- **`video_list` 通常只有第 1 个分P**;`re_num > 1` 时必须再调 getShowReplayList 展开,
  否则 2 小时以上的回放缺后半段。
- `video_list[].video_duration` 实测恒为 0,不可用;用 `video_str_duration`
  (`"113:21"` / `"02:00:02"` / `"16:47"`,按 `:` 从右往左时/分/秒累加解析)。
- `start_time` 是 Unix 秒,老数据为 0(publishedAt 需可空);`is_replay` 非零可过滤直播回放。
- `title` 含 `&amp;`、`&nbsp;` 等 HTML 实体,映射时要反转义。
- `time`/`date_format` 是展示串("昨天"),排序用 `start_time` 或接口默认顺序。

### 多分P展开

```http
GET https://v.douyu.com/wgapi/vod/center/getShowReplayList?vid={第一个分P的hash_id}&up_id={up_id}
```

`data.list[]`:`hash_id`(播放/弹幕 ID)、`vid`(数字 point_id,可用于签名)、
`video_duration`(格式化串)、`rank`。2h+ 场次切成多个 ~2h 分P,实测 4 分P场次返回 4 条;
第 2 个分P取流实测通过。

### 详情页(取 point_id)

```http
GET https://v.douyu.com/show/{hash_id}
```

HTML 内 `window.$DATA`(JS 对象字面量,非严格 JSON);正则 `"point_id":(\d+)` 可直接提取,
无需完整解析器。无 UA/Referer 也可拿。未找到 JSON 版详情接口(getVideoInfo 等 404)。

## 2. 可播流

链路(实测端到端通过):

1. 详情页(或列表/分P接口)取 `point_id`;
2. `GET https://www.douyu.com/swf_api/homeH5Enc?rids={point_id}` → crptext;
3. **与直播完全相同的签名**:`CRYPTO_JS + crptext → ub98484234(point_id, did, nowSec)`
   (第一个参数从 roomId 换成 point_id,did 用现有常量,无需登录);
4. 取流:

```http
POST https://v.douyu.com/wgapi/vodnc/front/stream/getStreamUrlWeb
Content-Type: application/x-www-form-urlencoded

{signed}&vid={hash_id}
```

未签名/签名错 → `{"error":7,"msg":"权限不足"}`。旧端点 `v.douyu.com/api/stream/getStreamUrl`
也能用但档位缺失(super 为空),**推荐 getStreamUrlWeb**。

响应 `data.thumb_video` 是「档位 key → 条目」映射,**key 动态**(normal/high/super/
1080p60/1440p60a…,老录像可能只剩 480P bit_rate=0),遍历所有 key、按 `bit_rate`
降序(接口顺序不保证高档在前)。每条含 `url`(m3u8)、`name`(档位名)、`bit_rate`、
`has_pms`、`now_free`。

m3u8 实测:VOD 形态(带 `#EXT-X-ENDLIST`)、无加密、分片普通 mp2t、分片时长累加与
详情页一致;**http→https 升级可用**(建议统一 https 防 renderer 混合内容);不要求
UA/Referer(带 referer 是保险项)。URL query 带短时凭证:`tlink` ≈2h、`tplay` ≈12h
(按时间戳推断,未等到实际过期)→ **不要持久化 Stream.url,播放失败/久挂后重新解析**。

`has_pms`/`now_free` 语义未确认,实测**没有阻止匿名播放**(71415 全档位 has_pms:1
仍可匿名拉 super);不要据此设 `membersOnly`。

## 3. VOD 弹幕

```http
GET https://v.douyu.com/wgapi/vod/center/getBarrageList?vid={hash_id}&start_time={ms}&end_time=-1
```

- 字段:`ctt`(文本)、`nn`(昵称)、`tl`(相对起点毫秒 → timeMs)、`col`(颜色索引)、
  `sts`(发送秒时间戳)。
- 分页:**单页上限 500,`data.count` 是当前页条数**;游标 = `data.end_time`,
  `end_time == -1` 即完;`start_time = 上一页 end_time` 循环。实测 113 分钟回放
  全量 8653 条 / 18 次请求 / 串行 8 秒 / 0 重复。窗口查询(显式 start/end)也支持。
- 颜色 `col`:实测出现 0、7–13;映射 0→#FFFFFF(或省略)、7→#FF5654、8→#FF7523、
  9→#FE69B3、10→#FFBC00、11→#78C946、12→#9E7FFF、13→#3D9BFF;1–6 未在 VOD 样本
  出现(可复用直播 6 色映射兜底,非实测)。
- 无 `mode` 字段,DanmakuItem.mode 省略。

## 4. 契约映射

- **新渠道** `douyu:replay`(独立于 `live:douyu`):kind=video,sourceInfoTpl
  `[{roomId, 直播间 ID, required}]`,pageable `{first:2, step:1}`(单房间维度翻页;
  多房间=多实例订阅)。hasMore 用 `page*20 < data.count`;total 省略(count 是场次数,
  分P展开后 item 数更多)。
- Item:id=`douyu:vod:{hash_id}`、url=`https://v.douyu.com/show/{hash_id}`、
  duration=parseDuration(video_str_duration)、publishedAt 仅 start_time>0、
  stream 留空(短时 token,playback 懒解析)、raw=原始分P。
- **routes**:`v.douyu.com/show/<hash>`(含 vmobile)→ kind=video,extract hash;
  现有直播路由不会误命中。
- **resolveVideoPlay(hash)**:详情页取 point_id → homeH5Enc → sign(point_id) →
  getStreamUrlWeb;Stream=`{url:https(...), format:"hls", quality:name}`,
  不设 rate(VOD 无切档语义)。
- **getDanmaku 分派**:`opts.kind === "vod" ? vod : live`,**默认保持 live**
  (douyu 现行为);URL 路由命中 show/ 时 feedops 会自动填 vod。
- **不需要新私有 op**;URL 路由 + resolveVideoPlay 覆盖播放器主路径。
  (可选:`douyu.replayList` 供 UI 免订阅直查。)
- ⚠️ **兼容性 caveat**:douyu 增加了 resolveVideoPlay 后,play 的 id-only 默认
  kind 从 live 变 video(platform-kit extension.ts playTarget 的「有 video resolver
  就默认 video」逻辑;Go feedops.go 同)。传 URL 或显式 kind:"live" 不受影响;
  仓库内未搜到 douyu 的 id-only .play 调用点,接入前再确认一次 UI 侧。

## 5. 风险与未验证项

- 签名脚本轮换(v=2201… 版本串):保留 JS 原文执行即可,**不要移植算法**。
- 非公开接口无 SLA;与 renmu123/douyu-video-cli(douyu-api)和 yt-dlp DouyuShowIE
  的路线吻合,端点已有生命周期。`getVodList` 等猜测端点 404,以本文为准。
- 未验证:token 实际过期行为、海外 IP 地区限制、频率阈值(18 连发未触发,建议串行+
  退避)、has_pms/now_free 确切语义、4K 档位、col=1–6、第 2+ 分P的弹幕(播放已验证)、
  正在直播的场次是否已入列表、m3u8 是否绑 IP/UA。

## 6. 工作量

| 项 | TS | Go |
| --- | --- | --- |
| 端点客户端+结构 | 0.5d | 0.5d |
| 渠道+映射 | 0.5d | 0.5d |
| 签名/取流接入(复用 JS 链) | 0.5d | 0.5d |
| VOD 弹幕分页+取消 | 0.5d | 0.5d |
| fixture 测试+真机烟测 | 0.5–1d | 0.5–1d |
| **合计** | **2–2.5 人日** | **2–2.5 人日**(TS 先行后 Go 可压到 1.5–2) |

MVP(单房间列表+播放,无分P/弹幕)每侧 0.5–1 人日,但不建议省分P展开。

## 7. 复现(只读)

```bash
curl -k "https://www.douyu.com/betard/71415"                                    # roomId→up_id
curl -k "https://v.douyu.com/wgapi/vod/center/authorShowVideoList?up_id=5JjALNBWgwXr&page=1&limit=20"
curl -k "https://v.douyu.com/wgapi/vod/center/getShowReplayList?vid=JmbBMkRlNzN740XA&up_id=5JjALNBWgwXr"
curl -k "https://v.douyu.com/show/85BAvqGOnLqMG4Lm"                             # point_id
curl -k "https://v.douyu.com/wgapi/vod/center/getBarrageList?vid=85BAvqGOnLqMG4Lm&start_time=0&end_time=-1"
# 取流:homeH5Enc(rids=point_id) → CRYPTO_JS+crptext 签名 →
curl -k -X POST -H "content-type: application/x-www-form-urlencoded" \
  --data-raw "${signed}&vid=85BAvqGOnLqMG4Lm" \
  "https://v.douyu.com/wgapi/vodnc/front/stream/getStreamUrlWeb"
```
