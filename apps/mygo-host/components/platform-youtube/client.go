// InnerTube 基础客户端(零登录):visitorData 获取、browse 请求、playability 判定
// 与直播元信息。player 请求体在 player.go。对照 lib/client.ts,网络全部走 sc.HTTP。

package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"mygo-host/platform"
)

const (
	youtubeiV1      = "https://www.youtube.com/youtubei/v1"
	youtubeiGapisV1 = "https://youtubei.googleapis.com/youtubei/v1"

	desktopChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	androidUA = "com.google.android.apps.youtube.vr.oculus/1.65.10 (Linux; U; Android 12L; " +
		"eureka-user Build/SQ3A.220605.009.A1) gzip"
	iosUA      = "com.google.ios.youtube/21.03.2(iPhone16,2; U; CPU iOS 18_7_2 like Mac OS X; en)"
	visionosUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 " +
		"(KHTML, like Gecko) Version/26.0 Safari/605.1.15"

	androidClientVersion  = "1.65.10"
	webClientVersion      = "2.20260120.01.00"
	iosClientVersion      = "21.03.2"
	iosDeviceModel        = "iPhone16,2"
	visionosClientVersion = "1.02"
	visionosDeviceModel   = "RealityDevice17,1"
	visionosOSVersion     = "26.5.23O471"
)

var visitorState struct {
	mu    sync.Mutex
	value string
	err   error
	wait  chan struct{}
}

func getVisitorData(ctx context.Context, sc *platform.ServiceContext) (string, error) {
	visitorState.mu.Lock()
	if visitorState.value != "" {
		value := visitorState.value
		visitorState.mu.Unlock()
		return value, nil
	}
	if wait := visitorState.wait; wait != nil {
		visitorState.mu.Unlock()
		<-wait
		visitorState.mu.Lock()
		defer visitorState.mu.Unlock()
		if visitorState.value != "" {
			return visitorState.value, nil
		}
		if visitorState.err != nil {
			return "", visitorState.err
		}
		return "", fmt.Errorf("YouTube visitorData 获取失败")
	}
	wait := make(chan struct{})
	visitorState.wait = wait
	visitorState.mu.Unlock()

	value, err := fetchVisitorData(ctx, sc)

	visitorState.mu.Lock()
	visitorState.value, visitorState.err = value, err
	visitorState.wait = nil
	visitorState.mu.Unlock()
	close(wait)
	return value, err
}

func fetchVisitorData(ctx context.Context, sc *platform.ServiceContext) (string, error) {
	body := map[string]any{
		"context": map[string]any{"client": map[string]any{
			"clientName": "ANDROID_VR", "clientVersion": androidClientVersion,
			"deviceMake": "Oculus", "deviceModel": "Quest 3", "androidSdkVersion": 32,
			"osName": "Android", "osVersion": "12L",
		}},
	}
	raw, err := postJSON(ctx, sc, youtubeiV1+"/visitor_id?prettyPrint=false", body, map[string]string{
		"user-agent": androidUA, "X-Goog-Api-Format-Version": "2",
	})
	if err != nil {
		return "", err
	}
	var res struct {
		ResponseContext *struct {
			VisitorData *string `json:"visitorData"`
		} `json:"responseContext"`
		VisitorData *string `json:"visitorData"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	value := derefString(res.VisitorData)
	if value == "" && res.ResponseContext != nil {
		value = derefString(res.ResponseContext.VisitorData)
	}
	if value == "" {
		return "", fmt.Errorf("YouTube visitorData 获取失败")
	}
	return value, nil
}

// postJSON POST JSON,非 2xx 报错(429 是限流/风控信号;宿主暂无治理层,不额外上报)。
func postJSON(ctx context.Context, sc *platform.ServiceContext, url string, body any, headers map[string]string) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	merged := map[string]string{"content-type": "application/json"}
	for key, value := range headers {
		merged[key] = value
	}
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL: url, Method: "POST", Headers: merged, Body: string(encoded),
	})
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("YouTube HTTP %d: %s", res.Status, url)
	}
	return []byte(res.Body), nil
}

type browseOptions struct {
	params       string
	continuation string
}

// youtubeBrowse POST 一次 InnerTube browse(WEB client + visitorData)。
func youtubeBrowse(ctx context.Context, sc *platform.ServiceContext, browseID string, opts browseOptions) (*jsonValue, error) {
	visitorData, err := getVisitorData(ctx, sc)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"context": map[string]any{"client": map[string]any{
			"clientName": "WEB", "clientVersion": webClientVersion,
			"hl": "en", "gl": "US", "visitorData": visitorData,
		}},
	}
	if opts.continuation != "" {
		body["continuation"] = opts.continuation
	} else {
		body["browseId"] = browseID
		if opts.params != "" {
			body["params"] = opts.params
		}
	}
	raw, err := postJSON(ctx, sc, youtubeiV1+"/browse?prettyPrint=false", body, map[string]string{
		"user-agent": desktopChromeUA,
	})
	if err != nil {
		return nil, err
	}
	return parseJSONValue(raw)
}

var loginRequiredRe = regexp.MustCompile(`(?i)login_required`)
var privateReasonRe = regexp.MustCompile(`(?i)private|age|members`)

func checkPlayability(res *playerResponse, videoID string) error {
	st := res.PlayabilityStatus
	if st != nil && derefString(st.Status) == "OK" {
		return nil
	}
	reason := "unknown"
	if st != nil {
		if st.Reason != nil {
			reason = *st.Reason
		} else if st.Status != nil {
			reason = *st.Status
		}
	}
	if loginRequiredRe.MatchString(reason) && st != nil && st.ErrorScreen != nil {
		if stReason := errorScreenSimpleText(st.ErrorScreen); stReason != "" && privateReasonRe.MatchString(stReason) {
			return fmt.Errorf("YouTube 视频不可播:%s", stReason)
		}
	}
	return fmt.Errorf("YouTube 视频不可播(%s): https://www.youtube.com/watch?v=%s", reason, videoID)
}

func errorScreenSimpleText(screen map[string]any) string {
	var node any = screen
	for _, key := range []string{"playerErrorMessageRenderer", "reason"} {
		obj, ok := node.(map[string]any)
		if !ok {
			return ""
		}
		node = obj[key]
	}
	obj, ok := node.(map[string]any)
	if !ok {
		return ""
	}
	text, _ := obj["simpleText"].(string)
	return text
}

// isLiveContent 直播判定:liveStreamability 存在即直播中;isLiveContent 兜底。
func isLiveContent(res *playerResponse) bool {
	if res == nil {
		return false
	}
	if res.PlayabilityStatus != nil && jsTruthy(res.PlayabilityStatus.LiveStreamability) {
		return true
	}
	vd := res.VideoDetails
	if vd == nil {
		return false
	}
	return (vd.IsLiveContent != nil && *vd.IsLiveContent) || (vd.IsLive != nil && *vd.IsLive)
}

// getYoutubeLiveMeta 取真实元信息;失败返回 nil(调用方回退占位,不阻塞订阅)。
func getYoutubeLiveMeta(ctx context.Context, sc *platform.ServiceContext, videoID string) *liveMeta {
	visitorData, err := getVisitorData(ctx, sc)
	if err != nil {
		return nil
	}
	res, err := getIosPlayerResponse(ctx, sc, videoID, visitorData)
	if err != nil {
		return nil
	}
	vd := res.VideoDetails
	if vd == nil {
		return nil
	}
	title := strings.TrimSpace(jsString(vd.Title))
	author := strings.TrimSpace(jsString(vd.Author))
	if title == "" && author == "" {
		return nil
	}
	var online *int
	if digits := digitsOnly(jsString(vd.ViewCount)); digits != "" {
		if value, err := strconv.Atoi(digits); err == nil && value > 0 {
			online = &value
		}
	}
	return &liveMeta{Title: title, Author: author, IsLive: isLiveContent(res), Online: online}
}

// jsString JS String(value) 的近似(主要处理字符串/数字/布尔/null)。
func jsString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func jsTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	default:
		return true
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
