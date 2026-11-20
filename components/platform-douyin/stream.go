// douyin 直播流解析:enter 响应的 stream_url → []platform.Stream。
// 支持 SDK(pull_data.stream_data + options.qualities)与 flv/hls_pull_url
// 兜底两种形态;最高清晰度排最前(产流侧契约)。三级取流链(enter → reflow →
// HTML)在 stream_fallback.go。

package douyin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mygo-host/platform"
)

// ipv6HostRe 丢弃 IPv6 主机的流(TS filterPlayable)。
var ipv6HostRe = regexp.MustCompile(`\[[0-9a-f:]+\]`)

// resolveLiveStreams 直播间取流:enter → reflow → HTML 三级降级。每次调用都取
// 新鲜直链(直链带过期时间)。enter 的成功响应必须复用给 reflow,避免同一房间
// 短时两次 enter(签名不同)拉高风控;第 1、2 级失败只记日志,HTML 失败才抛错。
func resolveLiveStreams(ctx context.Context, sc *platform.ServiceContext, webRID string) ([]platform.Stream, error) {
	if sc.JS == nil {
		return nil, fmt.Errorf("douyin 直播取流需要 JS 签名执行面(ABOGUS):%w", platform.ErrJSRunnerUnavailable)
	}
	id := strings.TrimSpace(webRID)
	if id == "" {
		return nil, errors.New("douyin: 缺少直播间 id")
	}
	var enterRes *enterResponse
	if room, err := fetchRoom(ctx, sc, id); err != nil {
		sc.Log("douyin enter API 失败: %v", err)
	} else {
		enterRes = &room
		if streams := parseStreams(room.room().StreamURL); len(streams) > 0 {
			return streams, nil
		}
	}
	if streams := resolveReflowStreams(ctx, sc, id, enterRes); len(streams) > 0 {
		return streams, nil
	}
	return resolveFromHTML(ctx, sc, id)
}

// parseStreams stream_url → 可播流(level 降序;stream_data 有效走 SDK 路径)。
func parseStreams(raw json.RawMessage) []platform.Stream {
	info := decodeStreamInfo(raw)
	pull := info.LiveCoreSDKData.PullData
	qualities := sortedQualities(pull.Options.Qualities)
	if sdk, ok := decodeSDKStreamData(pull.StreamData); ok {
		return filterPlayable(sdkStreams(sdk, qualities))
	}
	return filterPlayable(pullURLStreams(info, qualities))
}

func decodeStreamInfo(raw json.RawMessage) streamInfo {
	var info streamInfo
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &info)
	}
	return info
}

// sortedQualities 按 level 降序稳定排序(缺省 0)。
func sortedQualities(items []qualityItem) []qualityItem {
	out := append([]qualityItem(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return qualityLevel(out[i]) > qualityLevel(out[j])
	})
	return out
}

func qualityLevel(q qualityItem) int {
	n, ok := platform.ToInt(q.Level)
	if !ok {
		return 0
	}
	return int(n)
}

// qualityName name 为空回退 sdk_key。
func qualityName(q qualityItem) string {
	if name := platform.JSString(q.Name); name != "" {
		return name
	}
	return platform.JSString(q.SDKKey)
}

// sdkStreams SDK 路径:按档位取 stream_data.data[<sdk_key>].main。
func sdkStreams(data sdkStreamData, qualities []qualityItem) []platform.Stream {
	streams := []platform.Stream{}
	for _, q := range qualities {
		main := data.Data[platform.JSString(q.SDKKey)].Main
		streams = appendQualityStream(streams, q, platform.JSString(main.Flv), platform.JSString(main.Hls), true)
	}
	return streams
}

// pullURLStreams 兜底路径:Object.values(...)[len-level](倒序索引)。
func pullURLStreams(info streamInfo, qualities []qualityItem) []platform.Stream {
	flvList := orderedValues(info.FlvPullURL)
	hlsList := orderedValues(info.HlsPullURLMap)
	streams := []platform.Stream{}
	for _, q := range qualities {
		level := qualityLevel(q)
		streams = appendQualityStream(streams, q, reversedAt(flvList, level), reversedAt(hlsList, level), false)
	}
	return streams
}

// appendQualityStream 每档只出一条:flv 优先,hls 兜底;仅 SDK flv 带 authority。
func appendQualityStream(streams []platform.Stream, q qualityItem, flv, hls string, flvAuthority bool) []platform.Stream {
	quality, rate := qualityName(q), qualityLevel(q)
	if flv != "" {
		return append(streams, platform.Stream{
			URL: flv, Format: platform.Str("flv"), Headers: streamHeaders(flvAuthority),
			Quality: platform.Str(quality), Rate: platform.Int(rate),
		})
	}
	if hls != "" {
		return append(streams, platform.Stream{
			URL: hls, Format: platform.Str("hls"), Headers: streamHeaders(false),
			Quality: platform.Str(quality), Rate: platform.Int(rate),
		})
	}
	return streams
}

func streamHeaders(authority bool) map[string]string {
	headers := map[string]string{"referer": liveOrigin, "user-agent": uaEnter}
	if authority {
		headers["authority"] = liveOrigin
	}
	return headers
}

// reversedAt TS Object.values(...)[len-level] ?? ""。
func reversedAt(values []string, level int) string {
	index := len(values) - level
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

// orderedValues 按 JSON 键插入顺序取值(Go map 会丢顺序;非字符串按 "" 占位)。
func orderedValues(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil
	}
	values := []string{}
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return values
		}
		var value any
		if err := dec.Decode(&value); err != nil {
			return values
		}
		values = append(values, platform.JSString(value))
	}
	return values
}

// decodeSDKStreamData stream_data 仅当 trim 后以 "{" 开头且可解析才有效。
func decodeSDKStreamData(raw string) (sdkStreamData, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return sdkStreamData{}, false
	}
	var data sdkStreamData
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return sdkStreamData{}, false
	}
	return data, true
}

// filterPlayable 丢弃 IPv6 主机的流(TS filterPlayable)。
func filterPlayable(streams []platform.Stream) []platform.Stream {
	out := streams[:0]
	for _, stream := range streams {
		if !ipv6HostRe.MatchString(stream.URL) {
			out = append(out, stream)
		}
	}
	return out
}
