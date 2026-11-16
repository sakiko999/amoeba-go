// douyu 客户端:签名链(homeH5Enc 取 crptext → CryptoJS 经 sc.JS 执行)、
// getH5Play 逐 CDN 逐档位拉流、betard 房间信息。
// 对齐 TS lib/client.ts;签名 JS 原文执行,不移植成 Go。

package douyu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"mygo-host/platform"
)

const (
	clientUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
	// did 是斗鱼签名串里固定的设备 id(TS 同值)。
	did = "10000000000000000000000000001501"
)

// baseURL / mobileBaseURL 用 var 而非 const:离线测试用 httptest 覆写。
var (
	baseURL       = "https://www.douyu.com"
	mobileBaseURL = "https://m.douyu.com"
)

var httpURLRe = regexp.MustCompile(`(?i)^https?://`)

type quality struct {
	rate int
	name string
}

// signPayload 抓 homeH5Enc 的签名 payload(roomId → crptext JS blob)。
func signPayload(ctx context.Context, sc *platform.ServiceContext, roomID string) (string, error) {
	url := fmt.Sprintf("%s/swf_api/homeH5Enc?rids=%s", baseURL, roomID)
	raw, err := sc.HTTP.GetJSON(ctx, url, map[string]string{"user-agent": clientUA})
	if err != nil {
		return "", err
	}
	var res homeH5EncResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return "", err
	}
	if res.Data == nil {
		return "", nil
	}
	return jsString(res.Data["room"+roomID]), nil
}

// sign 运行 CryptoJS + crptext,调用 ub98484234(roomId, did, time) 得签名串。
func sign(ctx context.Context, sc *platform.ServiceContext, crptext, roomID string) (string, error) {
	if sc.JS == nil {
		return "", platform.ErrJSRunnerUnavailable
	}
	timeSec := sc.NowMs() / 1000
	code := cryptoJS + "\n" + crptext
	result, err := sc.JS.Call(ctx, code, "ub98484234", []any{roomID, did, timeSec})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	if s, ok := result.(string); ok {
		return s, nil
	}
	return jsString(result), nil
}

// roomInfo 房间信息(betard/{roomId});HTML 风控页报清晰错误,双编码 JSON 解一层。
func roomInfo(ctx context.Context, sc *platform.ServiceContext, roomID string) (*roomPayload, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:     baseURL + "/betard/" + roomID,
		Headers: map[string]string{"user-agent": clientUA, "referer": baseURL + "/" + roomID},
	})
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("douyu HTTP %d: betard/%s", res.Status, roomID)
	}
	if strings.HasPrefix(strings.TrimLeftFunc(res.Body, unicode.IsSpace), "<") {
		return nil, fmt.Errorf("douyu: 房间 %s 未返回房间信息(疑似风控,需浏览器环境验证)", roomID)
	}
	raw := json.RawMessage(res.Body)
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		raw = json.RawMessage(encoded)
	}
	var wrapper struct {
		Room json.RawMessage `json:"room"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	payload := raw
	if len(wrapper.Room) > 0 && string(wrapper.Room) != "null" {
		payload = wrapper.Room
	}
	var room roomPayload
	if err := json.Unmarshal(payload, &room); err != nil {
		return nil, err
	}
	return &room, nil
}

// h5PlayData POST getH5Play 返回原始 data(rtmp_url / rtmp_live / multirates 同源)。
func h5PlayData(ctx context.Context, sc *platform.ServiceContext, roomID, body string) (map[string]any, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:    baseURL + "/lapi/live/getH5Play/" + roomID,
		Method: "POST",
		Headers: map[string]string{
			"content-type": "application/x-www-form-urlencoded",
			"user-agent":   clientUA,
			"referer":      baseURL + "/" + roomID,
		},
		Body: body,
	})
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("douyu H5Play HTTP %d", res.Status)
	}
	if strings.TrimSpace(res.Body) == "" {
		return nil, nil
	}
	var payload h5PlayResponse
	if err := json.Unmarshal([]byte(res.Body), &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

// resolveH5Play 遍历所有 CDN 拉一档可播流(单个失败换下一个,返回首个可用)。
// body 不能带 ver=...&iar=1(服务端强制降档),只拼 &cdn=&rate=N。
func resolveH5Play(ctx context.Context, sc *platform.ServiceContext, roomID, signed string, rate int, cdns []string) (*platform.Stream, error) {
	list := cdns
	if len(list) == 0 {
		list = []string{""}
	}
	var lastErr error
	for _, cdn := range list {
		body := fmt.Sprintf("%s&cdn=%s&rate=%d", signed, cdn, rate)
		data, err := h5PlayData(ctx, sc, roomID, body)
		if err != nil {
			lastErr = err
			sc.Log("douyu: cdn=%s rate=%d 失败,换下一个: %v", cdn, rate, err)
			continue
		}
		base := jsString(data["rtmp_url"])
		live := htmlUnescape(jsString(data["rtmp_live"]))
		if base == "" || live == "" {
			lastErr = errors.New("no stream")
			continue
		}
		url := base + "/" + live
		format := "rtmp"
		if httpURLRe.MatchString(url) {
			format = "flv"
		}
		return &platform.Stream{
			URL:     url,
			Format:  platform.Str(format),
			Headers: map[string]string{"referer": baseURL + "/" + roomID, "user-agent": clientUA},
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("douyu H5Play: no stream for room %s", roomID)
	}
	return nil, lastErr
}

// parseQualities 列档位(rate=-1 的 multirates,按服务端顺序,高档在前)。
func parseQualities(data map[string]any) []quality {
	arr, _ := data["multirates"].([]any)
	out := make([]quality, 0, len(arr))
	for _, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rate, rateOK := toInt(m["rate"])
		name := jsString(m["name"])
		if !rateOK || name == "" {
			continue
		}
		out = append(out, quality{rate: int(rate), name: name})
	}
	return out
}

// parseCDNs 取可用 CDN;scdn 排后(dart 同款,scdn 优先兜底),稳定排序。
func parseCDNs(data map[string]any) []string {
	arr, _ := data["cdnsWithName"].([]any)
	cdns := make([]string, 0, len(arr))
	for _, entry := range arr {
		m, _ := entry.(map[string]any)
		cdn := jsString(m["cdn"])
		if cdn != "" {
			cdns = append(cdns, cdn)
		}
	}
	rank := func(cdn string) int {
		if strings.HasPrefix(cdn, "scdn") {
			return 1
		}
		return 0
	}
	sort.SliceStable(cdns, func(i, j int) bool { return rank(cdns[i]) < rank(cdns[j]) })
	return cdns
}

// resolveQualityStreams 逐档拉真实直链;首档失败整体报错,其余档失败跳过。
func resolveQualityStreams(ctx context.Context, sc *platform.ServiceContext, roomID, signed string, qualities []quality, cdns []string) ([]platform.Stream, error) {
	streams := make([]platform.Stream, 0, len(qualities))
	for idx, q := range qualities {
		stream, err := resolveH5Play(ctx, sc, roomID, signed, q.rate, cdns)
		if err != nil {
			if idx == 0 {
				return nil, err
			}
			sc.Log("douyu: 档位 %s(rate=%d) 解析失败,跳过: %v", q.name, q.rate, err)
			continue
		}
		stream.Quality = platform.Str(q.name)
		stream.Rate = platform.Int(q.rate)
		streams = append(streams, *stream)
	}
	return streams, nil
}

// resolveStreams 完整拉流:签名 → 列档位 + CDN → 逐档拉直链。
// 首档失败整体报错;其余档失败跳过;multirates 为空时 rate=-1 兜底单流。
func resolveStreams(ctx context.Context, sc *platform.ServiceContext, roomID string) ([]platform.Stream, error) {
	crptext, err := signPayload(ctx, sc, roomID)
	if err != nil {
		return nil, err
	}
	signed, err := sign(ctx, sc, crptext, roomID)
	if err != nil {
		return nil, err
	}
	listRes, err := h5PlayData(ctx, sc, roomID, signed+"&cdn=&rate=-1")
	if err != nil {
		return nil, err
	}
	qualities := parseQualities(listRes)
	cdns := parseCDNs(listRes)

	streams, err := resolveQualityStreams(ctx, sc, roomID, signed, qualities, cdns)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		stream, err := resolveH5Play(ctx, sc, roomID, signed, -1, cdns)
		if err != nil {
			return nil, err
		}
		streams = append(streams, *stream)
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("douyu H5Play: no stream for room %s", roomID)
	}
	return streams, nil
}
