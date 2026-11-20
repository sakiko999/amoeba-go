// douyin 取流的 reflow / HTML 两级兜底。2026-10 起 enter 响应不再含 stream_url,
// 真正的直链由 reflow/info 产出:复用 enter 响应里的 room.id_str(不再重抓),
// cookie/UA/referer 按 douyin.md B.7.2;reflow 失败再抓直播页 HTML 提取
// flv_pull_url。两级失败均只记日志,HTML 无流才向调用方抛错。

package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mygo-host/platform"
)

// reflowBase 用 var 便于离线测试覆写(与 liveBase 同套路)。
var reflowBase = "https://webcast.amemv.com"

// qualityOrder 上游固定键序(清晰度降序);ORIGION 是上游拼写,不要改。
var qualityOrder = []string{"FULL_HD1", "HD1", "SD1", "SD2", "ORIGION"}

var qualityNames = map[string]string{
	"FULL_HD1": "蓝光",
	"HD1":      "高清",
	"SD1":      "标清",
	"SD2":      "流畅",
	"ORIGION":  "原画",
}

// resolveReflowStreams 第二级:用 room 长号请求 reflow/info。enterRes 为 nil
// (enter 失败)时按 TS 语义补抓一次 enter;仍失败只记日志返回 nil。
func resolveReflowStreams(ctx context.Context, sc *platform.ServiceContext, webRID string, enterRes *enterResponse) []platform.Stream {
	res := enterRes
	if res == nil {
		room, err := fetchRoom(ctx, sc, webRID)
		if err != nil {
			sc.Log("douyin reflow: 获取 room 长号失败: %v", err)
			return nil
		}
		res = &room
	}
	longID := roomLongID(res.room())
	if longID == "" {
		sc.Log("douyin reflow: 无 room 长号")
		return nil
	}
	raw, err := sc.HTTP.GetJSON(ctx, reflowURL(longID), map[string]string{
		"user-agent": uaEnter,
		"referer":    liveOrigin,
		"authority":  "live.douyin.com",
		"cookie":     douyinCookie(ctx, sc),
	})
	if err != nil {
		sc.Log("douyin reflow API 失败,降级 HTML: %v", err)
		return nil
	}
	var body reflowResponse
	if err := platform.UnmarshalJSON(raw, &body); err != nil {
		sc.Log("douyin reflow API 失败,降级 HTML: %v", err)
		return nil
	}
	return parseReflowStreams(body.streamURL())
}

// roomLongID 对齐 TS:room.id_str ?? room.id,缺失时为空串。
func roomLongID(room enterRoom) string {
	if id := strings.TrimSpace(platform.JSString(room.IDStr)); id != "" {
		return id
	}
	return strings.TrimSpace(platform.JSString(room.ID))
}

func reflowURL(longID string) string {
	return reflowBase + "/webcast/room/reflow/info/?type_id=0&live_id=1&room_id=" +
		longID + "&sec_user_id=&version_code=99.99.99&app_id=6383"
}

// parseReflowStreams 按 QUALITY_ORDER 固定顺序产出;每档 flv 优先、hls 兜底,
// hls 字段先试 hls_pull_url_map 再试 hls_pull_url。
func parseReflowStreams(raw json.RawMessage) []platform.Stream {
	var pulls reflowPullURLs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &pulls)
	}
	hls := pulls.HlsPullURLMap
	if hls == nil {
		hls = pulls.HlsPullURL
	}
	streams := make([]platform.Stream, 0, len(qualityOrder))
	for idx, key := range qualityOrder {
		rate := len(qualityOrder) - idx
		if flv := pullURL(pulls.FlvPullURL[key]); flv != "" {
			streams = append(streams, fallbackStream(flv, "flv", key, rate))
			continue
		}
		if hlsURL := pullURL(hls[key]); hlsURL != "" {
			streams = append(streams, fallbackStream(hlsURL, "hls", key, rate))
		}
	}
	return filterPlayable(streams)
}

// escapedAmp URL 里可能出现的 JSON 二次转义「反斜杠 + u0026」。
// 用字节构造避免源码里再写一层转义。
var escapedAmp = string([]byte{92, 'u', '0', '0', '2', '6'})

// pullURL 取值并按 TS replaceAll 还原转义 &。
func pullURL(value any) string {
	return strings.ReplaceAll(platform.JSString(value), escapedAmp, "&")
}

func fallbackStream(rawURL, format, key string, rate int) platform.Stream {
	return platform.Stream{
		URL:     rawURL,
		Format:  platform.Str(format),
		Headers: fallbackHeaders(),
		Quality: platform.Str(reflowQualityName(key)),
		Rate:    platform.Int(rate),
	}
}

func reflowQualityName(key string) string {
	if name := qualityNames[key]; name != "" {
		return name
	}
	return key
}

func fallbackHeaders() map[string]string {
	return map[string]string{
		"referer":    liveOrigin,
		"user-agent": uaEnter,
		"authority":  "live.douyin.com",
	}
}

// resolveFromHTML 第三级:抓直播页 HTML 提取 flv_pull_url(非 2xx 直接上抛)。
func resolveFromHTML(ctx context.Context, sc *platform.ServiceContext, webRID string) ([]platform.Stream, error) {
	pageURL := liveBase + "/" + webRID
	html, err := sc.HTTP.GetText(ctx, pageURL, map[string]string{
		"user-agent": uaEnter,
		"referer":    pageURL,
		"authority":  "live.douyin.com",
		"cookie":     douyinCookie(ctx, sc),
	})
	if err != nil {
		return nil, err
	}
	streams := parseHTMLPullStreams(html)
	if len(streams) == 0 {
		return nil, fmt.Errorf("douyin HTML: 未找到可播流(房间 %s 可能未开播)", webRID)
	}
	return streams, nil
}

// parseHTMLPullStreams 在 flv_pull_url 片段里按清晰度找 `\"KEY\":\"http...`。
// 只解析 flv(hls 是 m3u8,另有链路);转义引号是结束边界,`&` 还原为 &。
func parseHTMLPullStreams(html string) []platform.Stream {
	start := strings.Index(html, "flv_pull_url")
	if start < 0 {
		return nil
	}
	const key = "flv_pull_url"
	end := start + 3000
	if rel := strings.Index(html[start+len(key):], "_pull_url"); rel >= 0 {
		end = start + len(key) + rel
	}
	if end > len(html) {
		end = len(html)
	}
	seg := html[start:end]
	streams := make([]platform.Stream, 0, len(qualityOrder))
	for idx, q := range qualityOrder {
		url := htmlStreamURL(seg, q)
		if url == "" {
			continue
		}
		streams = append(streams, fallbackStream(pullURL(url), "flv", q, len(qualityOrder)-idx))
	}
	return filterPlayable(streams)
}

// htmlStreamURL 找 `\"Q\":\"` 后的首个 http(s) 地址,直到 `\"` 边界。
func htmlStreamURL(seg, quality string) string {
	at := strings.Index(seg, `\"`+quality+`\":\"`)
	if at < 0 {
		return ""
	}
	rest := seg[at:]
	urlStart := strings.Index(rest, "http:")
	if urlStart < 0 {
		urlStart = strings.Index(rest, "https:")
	}
	if urlStart < 0 {
		return ""
	}
	var b strings.Builder
	for i := urlStart; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) && rest[i+1] == '"' {
			break
		}
		b.WriteByte(rest[i])
	}
	return b.String()
}
