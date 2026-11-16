// 直播可播流解析:profileRoom API 一次给全「线路 × 档位」,multiLine.url 已含
// 签名(无需 buildAntiCode);HLS 用 ratio 切档,flv 仅最高档;每档展开各 CDN,
// 最高清晰度在前(档位原序),player 按 rate 去重后自然选到最高档。

package huya

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// playback 平台播放/弹幕能力(注册进 feed 协议)。
func playback(sc *platform.ServiceContext) *platform.Playback {
	return &platform.Playback{
		Platform: "huya",
		Routes:   []platform.Route{{Re: routeTestRe, Kind: "live", ExtractID: extractRoomID}},
		ResolveLivePlay: func(ctx context.Context, id string, _ platform.ResolveInfo) ([]platform.Stream, error) {
			return resolveLivePlay(ctx, sc, id)
		},
		GetDanmaku: func(id string, _ platform.DanmakuInfo) platform.DanmakuStream {
			return danmakuStream(sc, id)
		},
	}
}

var (
	// routeTestRe 匹配 item.url 形态(与 TS playback.routes 的 test 一致)。
	routeTestRe = regexp.MustCompile(`(?:^|[./])huya\.com/(\d+)`)
	routeIDRe   = regexp.MustCompile(`huya\.com/(\d+)`)
)

func extractRoomID(url string) (string, bool) {
	m := routeIDRe.FindStringSubmatch(url)
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// resolveLivePlay 懒解析;未开播/无线路报错(与 TS 文案一致)。
func resolveLivePlay(ctx context.Context, sc *platform.ServiceContext, roomID string) ([]platform.Stream, error) {
	data, err := fetchProfileRoom(ctx, sc, roomID)
	if err != nil {
		return nil, err
	}
	return playStreams(roomID, data)
}

// apiLine 按 CDN 归拢的一条线路(flv/hls 各一个完整签名直链)。
type apiLine struct{ Cdn, FlvURL, HlsURL string }

// apiBitRate 档位(名称 + iBitRate;0 = 原始最高档)。
type apiBitRate struct {
	Name    string
	BitRate int
}

// parseProfile 提取线路(flv 顺序优先、hls 补充新 CDN)与档位(原序,去 HDR)。
func parseProfile(data *profileRoomData) ([]apiLine, []apiBitRate) {
	order := []string{}
	byCDN := map[string]*apiLine{}
	touch := func(cdn string) *apiLine {
		if line, ok := byCDN[cdn]; ok {
			return line
		}
		line := &apiLine{Cdn: cdn}
		byCDN[cdn] = line
		order = append(order, cdn)
		return line
	}
	if data.Stream != nil {
		collectLines(data.Stream.Flv, touch, func(line *apiLine, url string) { line.FlvURL = url })
		collectLines(data.Stream.Hls, touch, func(line *apiLine, url string) { line.HlsURL = url })
	}
	lines := make([]apiLine, 0, len(order))
	for _, cdn := range order {
		if line := byCDN[cdn]; line.FlvURL != "" || line.HlsURL != "" {
			lines = append(lines, *line)
		}
	}
	return lines, parseBitRates(data.LiveData)
}

func collectLines(group *streamGroup, touch func(string) *apiLine, set func(*apiLine, string)) {
	if group == nil {
		return
	}
	for _, item := range group.MultiLine {
		cdn, url := strVal(item.CdnType), strVal(item.URL)
		if cdn != "" && url != "" {
			set(touch(cdn), url)
		}
	}
}

func parseBitRates(live *profileLive) []apiBitRate {
	if live == nil || live.BitRateInfo == nil {
		return nil
	}
	rates := make([]apiBitRate, 0, len(*live.BitRateInfo))
	for _, row := range *live.BitRateInfo {
		name := strVal(row.Name)
		if name == "" || strings.Contains(name, "HDR") {
			continue
		}
		rates = append(rates, apiBitRate{Name: name, BitRate: numVal(row.BitRate)})
	}
	return rates
}

// playStreams 线路 × 档位 → Stream 列表(无档位时单流,HLS 优先、flv 兜底)。
func playStreams(roomID string, data *profileRoomData) ([]platform.Stream, error) {
	lines, rates := parseProfile(data)
	if len(lines) == 0 {
		return nil, fmt.Errorf("huya: no stream for room %s(未开播或无线路)", roomID)
	}
	headers := map[string]string{"referer": mHuya + "/" + roomID, "user-agent": playUA}
	if len(rates) == 0 {
		line := lines[0]
		url, format := line.HlsURL, "hls"
		if url == "" {
			url, format = line.FlvURL, "flv"
		}
		if url == "" {
			return []platform.Stream{}, nil
		}
		return []platform.Stream{{URL: url, Format: platform.Str(format), Headers: headers}}, nil
	}
	streams := make([]platform.Stream, 0, len(rates)*len(lines)*2)
	for i, rate := range rates {
		top := i == 0
		for _, line := range lines {
			if line.HlsURL != "" {
				streams = append(streams, platform.Stream{
					URL:     withRatio(line.HlsURL, rate.BitRate),
					Format:  platform.Str("hls"),
					Headers: headers,
					Quality: platform.Str(rate.Name + "·" + line.Cdn),
					Rate:    platform.Int(rate.BitRate),
				})
			}
			if top && line.FlvURL != "" {
				streams = append(streams, platform.Stream{
					URL:     line.FlvURL,
					Format:  platform.Str("flv"),
					Headers: headers,
					Quality: platform.Str(rate.Name + "·" + line.Cdn),
					Rate:    platform.Int(rate.BitRate),
				})
			}
		}
	}
	return streams, nil
}

var (
	ratioPresentRe = regexp.MustCompile(`[?&]ratio=\d+`)
	ratioValueRe   = regexp.MustCompile(`([?&]ratio=)\d+`)
)

// withRatio 覆盖/追加 HLS 的 ratio 档位;bitRate=0 不传(CDN 拒绝 ratio=0)。
func withRatio(url string, bitRate int) string {
	if url == "" || bitRate == 0 {
		return url
	}
	if ratioPresentRe.MatchString(url) {
		return ratioValueRe.ReplaceAllString(url, "${1}"+strconv.Itoa(bitRate))
	}
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return url + sep + "ratio=" + strconv.Itoa(bitRate)
}
