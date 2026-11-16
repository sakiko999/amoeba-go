// usher master(HLS)解析:EXT-X-STREAM-INF → 档位列表,并显式按
// 高度/带宽降序重排(master 原始顺序不是高→低,播放器选流契约取 streams[0])。

package twitch

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

var (
	resolutionRe = regexp.MustCompile(`^\d{2,5}x(\d{2,5})$`)
	attrRe       = regexp.MustCompile(`([A-Za-z0-9-]+)=("[^"]*"|[^,]*)`)
	audioOnlyRe  = regexp.MustCompile(`(?i)audio_only`)
	urlQualityRe = regexp.MustCompile(`(?i)(?:^|/)(\d{3,4})p(\d{1,3})?(?:/|$)`)
)

type twitchVariant struct {
	URL       string
	Bandwidth int
	Height    *int
	FrameRate *float64
	Quality   string
}

// parseTwitchMaster 解析 master(纯函数);无档位返回空切片。
func parseTwitchMaster(master string) []twitchVariant {
	lines := strings.Split(master, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	variants := make([]twitchVariant, 0, 4)
	for i, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		if variant, ok := parseStreamInf(line, lines[i+1:]); ok {
			variants = append(variants, variant)
		}
	}
	sort.SliceStable(variants, func(i, j int) bool {
		hi, hj := variantHeight(variants[i]), variantHeight(variants[j])
		if hi != hj {
			return hi > hj
		}
		return variants[i].Bandwidth > variants[j].Bandwidth
	})
	return variants
}

// parseStreamInf 解析一条 STREAM-INF 与其后的 variant URL(下一非空非注释行)。
func parseStreamInf(line string, rest []string) (twitchVariant, bool) {
	attrs := parseAttrList(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
	variantURL := ""
	for _, next := range rest {
		if next == "" || strings.HasPrefix(next, "#") {
			continue
		}
		variantURL = next
		break
	}
	if variantURL == "" {
		return twitchVariant{}, false
	}
	height, hasHeight := resolutionHeight(attrs["RESOLUTION"])
	rawFrameRate, hasFrameRateAttr := attrs["FRAME-RATE"]
	frameRate, hasFrameRate := frameRateValue(rawFrameRate, hasFrameRateAttr)
	variant := twitchVariant{
		URL:       variantURL,
		Bandwidth: bandwidthValue(attrs["BANDWIDTH"]),
		Quality:   variantQuality(attrs["NAME"], attrs["IVS-NAME"], variantURL, height, hasHeight, frameRate, hasFrameRate),
	}
	if hasHeight {
		variant.Height = platform.Int(height)
	}
	if hasFrameRate {
		variant.FrameRate = platform.Float64(frameRate)
	}
	return variant, true
}

// twitchStreamsFromMaster master → Stream[](最高档在前,audio_only 最后)。
func twitchStreamsFromMaster(master string) ([]platform.Stream, error) {
	variants := parseTwitchMaster(master)
	if len(variants) == 0 {
		return nil, fmt.Errorf("twitch: master 播放列表无可用档位")
	}
	streams := make([]platform.Stream, 0, len(variants))
	for _, variant := range variants {
		stream := platform.Stream{
			URL:     variant.URL,
			Format:  platform.Str("hls"),
			Quality: platform.Str(variant.Quality),
		}
		if variant.Height != nil {
			stream.Rate = platform.Int(*variant.Height)
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

// parseAttrList 解析一行属性表(引号内的逗号不切分)。
func parseAttrList(raw string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrRe.FindAllStringSubmatch(raw, -1) {
		value := match[2]
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}
		attrs[match[1]] = value
	}
	return attrs
}

// variantQuality 优先 NAME/IVS-NAME,其次 URL 里的 <height>p<fps>,再按 RESOLUTION。
func variantQuality(name, ivsName, variantURL string, height int, hasHeight bool, frameRate float64, hasFrameRate bool) string {
	selected := name
	if selected == "" {
		selected = ivsName
	}
	if trimmed := strings.TrimSpace(selected); trimmed != "" {
		return trimmed
	}
	path := variantURL
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	if audioOnlyRe.MatchString(path) {
		return "audio_only"
	}
	if match := urlQualityRe.FindStringSubmatch(path); match != nil {
		if match[2] != "" {
			return match[1] + "p" + match[2]
		}
		return match[1] + "p"
	}
	if hasHeight {
		if hasFrameRate {
			return fmt.Sprintf("%dp%d", height, int(math.Round(frameRate)))
		}
		return fmt.Sprintf("%dp", height)
	}
	return "audio_only"
}

func resolutionHeight(raw string) (int, bool) {
	match := resolutionRe.FindStringSubmatch(raw)
	if match == nil {
		return 0, false
	}
	height, err := strconv.Atoi(match[1])
	if err != nil || height <= 0 {
		return 0, false
	}
	return height, true
}

// bandwidthValue 非有限值/缺失回落 0(对齐 Number.isFinite 兜底)。
func bandwidthValue(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(value)
}

// frameRateValue 缺失/不可解析 → undefined;显式空串按 JS Number("") = 0 处理。
func frameRateValue(raw string, present bool) (float64, bool) {
	if !present {
		return 0, false
	}
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func variantHeight(variant twitchVariant) int {
	if variant.Height == nil {
		return -1
	}
	return *variant.Height
}
