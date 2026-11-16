// 可播流解析:VISIONOS → ANDROID_VR → WEB 三级 player 尝试、直播 HLS 优先、
// DASH 装配(dash.go)与渐进式兜底。与 client.ts 的 resolveYoutubeStreams 对齐
// (含既有取舍:resolveFormatUrl 只认 url/cipher,baseUrl-only 的流会跳过)。

package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"mygo-host/platform"
)

func resolveYoutubeStreams(ctx context.Context, sc *platform.ServiceContext, videoID string) ([]platform.Stream, error) {
	visitorData, err := getVisitorData(ctx, sc)
	if err != nil {
		return nil, err
	}
	res, err := fetchPlayerResponseWithFallback(ctx, sc, videoID, visitorData)
	if err != nil {
		return nil, err
	}
	if streams := resolveLiveStreams(ctx, sc, res, videoID, visitorData); streams != nil {
		return streams, nil
	}
	return resolveFromStreamingData(ctx, sc, res, videoID)
}

// fetchPlayerResponseWithFallback VISIONOS → ANDROID_VR → WEB,任一成功且可播即返回。
func fetchPlayerResponseWithFallback(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (*playerResponse, error) {
	var lastErr error
	attempt := func(fn func() (*playerResponse, error)) *playerResponse {
		res, err := fn()
		if err != nil {
			lastErr = err
			return nil
		}
		if err := checkPlayability(res, videoID); err != nil {
			lastErr = err
			return nil
		}
		return res
	}
	clients := []func() (*playerResponse, error){
		func() (*playerResponse, error) { return getVisionosPlayerResponse(ctx, sc, videoID, visitorData) },
		func() (*playerResponse, error) { return getAndroidPlayerResponse(ctx, sc, videoID, visitorData) },
		func() (*playerResponse, error) { return getWebPlayerResponse(ctx, sc, videoID, visitorData) },
	}
	for _, client := range clients {
		if res := attempt(client); res != nil {
			return res, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("YouTube 无法获取 playerResponse: %s", videoID)
}

// resolveLiveStreams 直播:VR/WEB 自带 hls 优先,否则 iOS client 兜底;非直播返回 nil。
func resolveLiveStreams(ctx context.Context, sc *platform.ServiceContext, res *playerResponse, videoID, visitorData string) []platform.Stream {
	if !isLiveContent(res) {
		return nil
	}
	if hls := res.StreamingData.hls(); hls != "" {
		return []platform.Stream{hlsStream(hls, iosUA)}
	}
	iosHls, err := iosLiveHLS(ctx, sc, videoID, visitorData)
	if err != nil {
		sc.Log("youtube: iOS live 请求失败: %v", err)
		return nil
	}
	if iosHls != "" {
		return []platform.Stream{hlsStream(iosHls, iosUA)}
	}
	return nil
}

// resolveFromStreamingData DASH 全档位优先,失败回退渐进式 mp4,最后兜底 HLS。
func resolveFromStreamingData(ctx context.Context, sc *platform.ServiceContext, res *playerResponse, videoID string) ([]platform.Stream, error) {
	headers := map[string]string{"referer": "https://www.youtube.com/", "user-agent": desktopChromeUA}
	duration := 0.0
	if res.VideoDetails != nil {
		duration = jsNumber(res.VideoDetails.LengthSeconds)
	}
	dash, err := youtubeDashStreams(ctx, sc, res.StreamingData.adaptiveFormats(), duration, headers, videoID)
	if err != nil {
		return nil, err
	}
	if len(dash) > 0 {
		return dash, nil
	}
	if best := pickProgressiveVideo(res.StreamingData.formats()); best != nil {
		if streamURL, ok := resolveFormatURL(ctx, sc, *best, videoID); ok {
			return []platform.Stream{{URL: streamURL, Format: platform.Str("mp4"), Headers: headers}}, nil
		}
	}
	if hls := res.StreamingData.hls(); hls != "" {
		return []platform.Stream{hlsStream(hls, "")}, nil
	}
	return nil, fmt.Errorf("YouTube 未找到可播直链: https://www.youtube.com/watch?v=%s", videoID)
}

func iosLiveHLS(ctx context.Context, sc *platform.ServiceContext, videoID, visitorData string) (string, error) {
	ios, err := getIosPlayerResponse(ctx, sc, videoID, visitorData)
	if err != nil {
		return "", err
	}
	if err := checkPlayability(ios, videoID); err != nil {
		return "", err
	}
	return ios.StreamingData.hls(), nil
}

func hlsStream(streamURL, userAgent string) platform.Stream {
	headers := map[string]string{"referer": "https://www.youtube.com/"}
	if userAgent != "" {
		headers["user-agent"] = userAgent
	}
	return platform.Stream{URL: streamURL, Format: platform.Str("hls"), Headers: headers}
}

func (s *streamingData) formats() []playerFormat {
	if s == nil {
		return nil
	}
	return s.Formats
}

func (s *streamingData) adaptiveFormats() []playerFormat {
	if s == nil {
		return nil
	}
	return s.AdaptiveFormats
}

func jsNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		number := strings.TrimSpace(v)
		if number == "" {
			return 0
		}
		parsed, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

// pickProgressiveVideo 渐进式 mp4(音视频合一)里挑最高的一档。
// 高度取 itag 表(与 TS 一致,不依赖响应里的 height);
// 相等高度保留先出现者(对齐 ramda maxBy(height, f, best) 语义)。
func pickProgressiveVideo(formats []playerFormat) *playerFormat {
	var best *playerFormat
	for _, format := range formats {
		if !isPlayableProgressive(format) {
			continue
		}
		info, _ := getItag(format.itagID())
		if best == nil {
			candidate := format
			best = &candidate
			continue
		}
		bestInfo, _ := getItag(best.itagID())
		if info.height > bestInfo.height {
			candidate := format
			best = &candidate
		}
	}
	return best
}

func isPlayableProgressive(format playerFormat) bool {
	info, ok := getItag(format.itagID())
	if !ok || !isPlayableItag(info) {
		return false
	}
	if info.kind != itagVideo {
		return false
	}
	if format.url() == "" && derefString(format.Cipher) == "" && derefString(format.SignatureCipher) == "" {
		return false
	}
	return true
}

// resolveFormatURL 解出 format 的可播 URL(签名+ n 参数);失败 ok=false。
func resolveFormatURL(ctx context.Context, sc *platform.ServiceContext, format playerFormat, videoID string) (string, bool) {
	streamURL := format.url()
	if streamURL == "" {
		cipherStr := derefString(format.Cipher)
		if cipherStr == "" {
			cipherStr = derefString(format.SignatureCipher)
		}
		if cipherStr == "" {
			return "", false
		}
		cipher := parseCipher(cipherStr)
		streamURL = cipher["url"]
		signatureParam := cipher["sp"]
		if signatureParam == "" {
			signatureParam = "sig"
		}
		if cipher["s"] != "" && !strings.Contains(streamURL, "&"+signatureParam+"=") {
			return "", false
		}
	}
	if streamURL == "" {
		return "", false
	}
	if hasThrottlingParam(streamURL) {
		deobfuscated, err := deobfuscateNParam(ctx, sc, streamURL, videoID)
		if err != nil {
			return "", false
		}
		streamURL = deobfuscated
	}
	return streamURL, true
}

func parseCipher(cipherStr string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(cipherStr, "&") {
		key, value, found := strings.Cut(part, "=")
		if !found || key == "" {
			continue
		}
		decoded, err := url.PathUnescape(value)
		if err != nil {
			decoded = value
		}
		out[key] = decoded
	}
	return out
}
