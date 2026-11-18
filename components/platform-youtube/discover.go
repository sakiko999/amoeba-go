// YouTube 直播页(Live tab)匿名解析:直播热门 / 栏目分区 / 栏目目录。
// 端点 browse + browseId=UC4R8DWoMoI7CAwX8_LjQHig,响应是若干栏目;只保留带
// LIVE badge 的条目(Recent/Upcoming 解析成空),整页结果做 5 分钟缓存(并发去重)。

package youtube

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"mygo-host/platform"
)

const (
	livePageBrowseID     = "UC4R8DWoMoI7CAwX8_LjQHig"
	liveShelvesTTLMillis = 5 * 60 * 1000
)

var (
	liveNowRe       = regexp.MustCompile(`(?i)^live now`)
	leadingSepRe    = regexp.MustCompile(`^[\s-]+`)
	slugInvalidRe   = regexp.MustCompile(`[^a-z0-9]+`)
	slugEdgesRe     = regexp.MustCompile(`^-+|-+$`)
	categorySlugRe  = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
	watchingTextRe  = regexp.MustCompile(`(?i)watching`)
	viewersNumberRe = regexp.MustCompile(`(?i)([\d.,]+)\s*([KMB])?`)
)

type liveShelvesCall struct {
	at      int64
	done    chan struct{}
	shelves []youtubeLiveShelf
	err     error
}

var liveShelvesState struct {
	mu    sync.Mutex
	cache *liveShelvesCall
}

// fetchYoutubeLiveShelves 拉直播页并解析;5 分钟内复用(并发调用只发一次请求)。
func fetchYoutubeLiveShelves(ctx context.Context, sc *platform.ServiceContext) ([]youtubeLiveShelf, error) {
	liveShelvesState.mu.Lock()
	if call := liveShelvesState.cache; call != nil && sc.NowMs()-call.at < liveShelvesTTLMillis {
		liveShelvesState.mu.Unlock()
		<-call.done
		return call.shelves, call.err
	}
	call := &liveShelvesCall{at: sc.NowMs(), done: make(chan struct{})}
	liveShelvesState.cache = call
	liveShelvesState.mu.Unlock()

	raw, err := youtubeBrowse(ctx, sc, livePageBrowseID, browseOptions{})
	var shelves []youtubeLiveShelf
	if err == nil {
		shelves = parseYoutubeLiveShelves(raw)
		if len(shelves) == 0 {
			err = fmt.Errorf("YouTube 直播页没有可用栏目（页面结构可能已变）")
		}
	}
	call.shelves, call.err = shelves, err
	if err != nil {
		liveShelvesState.mu.Lock()
		if liveShelvesState.cache == call {
			liveShelvesState.cache = nil
		}
		liveShelvesState.mu.Unlock()
	}
	close(call.done)
	return shelves, err
}

// parseYoutubeLiveShelves 直播页原始 JSON → 栏目数组(纯函数)。兼容实测的
// richSectionRenderer.content.richShelfRenderer 与旧版直接 richShelfRenderer。
func parseYoutubeLiveShelves(raw *jsonValue) []youtubeLiveShelf {
	tabContent := raw.path("contents", "twoColumnBrowseResultsRenderer", "tabs").
		firstItem().path("tabRenderer", "content")
	sections := tabContent.path("richGridRenderer", "contents").items()
	if sections == nil {
		sections = tabContent.path("sectionListRenderer", "contents").items()
	}
	out := []youtubeLiveShelf{}
	for _, section := range sections {
		shelf := section.path("richSectionRenderer", "content", "richShelfRenderer")
		if shelf == nil {
			shelf = section.get("richShelfRenderer")
		}
		if shelf == nil {
			continue
		}
		title, ok := ytText(shelf.get("title"))
		if !ok || title == "" {
			continue
		}
		hits := []youtubeLiveHit{}
		for _, item := range shelf.get("contents").items() {
			videoRenderer := item.path("richItemRenderer", "content", "videoRenderer")
			if videoRenderer == nil || !hasLiveBadge(videoRenderer.get("thumbnailOverlays")) {
				continue
			}
			if hit, ok := videoRendererToHit(videoRenderer); ok {
				hits = append(hits, hit)
			}
		}
		out = append(out, youtubeLiveShelf{Title: title, Slug: youtubeShelfSlug(title), Hits: hits})
	}
	return out
}

func videoRendererToHit(videoRenderer *jsonValue) (youtubeLiveHit, bool) {
	videoID, _ := videoRenderer.get("videoId").asText()
	if videoID == "" {
		return youtubeLiveHit{}, false
	}
	title := videoID
	if text, ok := ytText(videoRenderer.get("title")); ok {
		title = text
	}
	// TS `textOf(ownerText) ?? textOf(longBylineText)`:?? 只对 undefined
	// 回退,ownerText 解析为空串时保留空串(ok=true)不回退。
	channelTitle, ok := ytText(videoRenderer.get("ownerText"))
	if !ok {
		channelTitle, _ = ytText(videoRenderer.get("longBylineText"))
	}
	viewersText, _ := ytText(videoRenderer.get("viewCountText"))
	return youtubeLiveHit{
		VideoID:      videoID,
		Title:        title,
		ChannelTitle: channelTitle,
		Thumb:        ytThumb(videoRenderer.get("thumbnail")),
		Viewers:      viewersOf(viewersText),
	}, true
}

// hasLiveBadge thumbnailOverlays 里有时间状态文本为 LIVE。
func hasLiveBadge(overlays *jsonValue) bool {
	for _, overlay := range overlays.items() {
		text, ok := ytText(overlay.path("thumbnailOverlayTimeStatusRenderer", "text"))
		if ok && strings.ToUpper(text) == "LIVE" {
			return true
		}
	}
	return false
}

// viewersOf 只认 "1,234 watching"(缩写 1.2K 按倍数还原);"views"/"waiting" 不取数。
func viewersOf(text string) *int {
	if text == "" || !watchingTextRe.MatchString(text) {
		return nil
	}
	match := viewersNumberRe.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	base, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil || !(base > 0) {
		return nil
	}
	multiplier := 1.0
	switch strings.ToUpper(match[2]) {
	case "K":
		multiplier = 1e3
	case "M":
		multiplier = 1e6
	case "B":
		multiplier = 1e9
	}
	value := int(math.Round(base * multiplier))
	return &value
}

func youtubeShelfSlug(title string) string {
	trimmed := strings.TrimSpace(title)
	if !liveNowRe.MatchString(trimmed) {
		return slugify(trimmed)
	}
	rest := liveNowRe.ReplaceAllString(trimmed, "")
	rest = leadingSepRe.ReplaceAllString(rest, "")
	rest = strings.TrimSpace(rest)
	if rest != "" {
		return slugify(rest)
	}
	return "hot"
}

func slugify(text string) string {
	slug := slugInvalidRe.ReplaceAllString(strings.ToLower(text), "-")
	return slugEdgesRe.ReplaceAllString(slug, "")
}

// normalizeYoutubeLiveCategory 用户输入(裸 slug / 栏目名 / 完整栏目名)→ slug。
func normalizeYoutubeLiveCategory(input string) string {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ""
	}
	slug := slugify(raw)
	if liveNowRe.MatchString(raw) {
		slug = youtubeShelfSlug(raw)
	}
	if !categorySlugRe.MatchString(slug) {
		return ""
	}
	return slug
}
