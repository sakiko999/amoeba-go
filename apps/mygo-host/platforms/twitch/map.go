// 上游对象 → feed 契约映射(纯函数,fixture 测试覆盖):
// 频道元数据/流列表 → Live,分类榜 → Category,频道搜索 → Video。

package twitch

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"mygo-host/platform"
)

// thumbnailFor 离线频道也返回的预览占位图。
func thumbnailFor(login string) string {
	return "https://static-cdn.jtvnw.net/previews-ttv/live_user_" + login + "-640x360.jpg"
}

// twitchLiveItem 单频道 → Live(离线仍产出,避免下播后订阅清空)。
func twitchLiveItem(user *gqlUser, login string, fetchedAt int64) platform.Item {
	stream := user.Stream
	displayName := deref(user.DisplayName)
	if displayName == "" {
		displayName = login
	}
	item := platform.Item{
		ID:        "twitch:" + login,
		SourceID:  "twitch:live",
		Kind:      platform.KindLive,
		FetchedAt: fetchedAt,
		URL:       platform.Str("https://www.twitch.tv/" + login),
		Thumbnail: platform.Str(thumbnailFor(login)),
		Author:    &platform.Author{Name: displayName},
		Platform:  "twitch",
		RoomID:    platform.Str(login),
	}
	if stream != nil {
		live := platform.LiveStatusLive
		item.LiveStatus = &live
		item.Online = platform.Int(intOrZero(stream.ViewersCount))
		item.Title = firstNonEmpty(deref(stream.Title), displayName)
		item.ShowTime = showTimeOrNil(stream.CreatedAt)
	} else {
		offline := platform.LiveStatusOffline
		item.LiveStatus = &offline
		item.Online = platform.Int(0)
		item.Title = displayName + "（离线）"
	}
	if description := deref(user.Description); description != "" {
		item.Introduction = platform.Str(description)
	}
	return item
}

// twitchStreamToLive 流列表条目 → Live;缺 broadcaster.login 的脏数据返回 nil。
func twitchStreamToLive(stream gqlStreamNode, sourceID string, fetchedAt int64) *platform.Item {
	login, displayName := "", ""
	if stream.Broadcaster != nil {
		login = strings.ToLower(strings.TrimSpace(deref(stream.Broadcaster.Login)))
		displayName = strings.TrimSpace(deref(stream.Broadcaster.DisplayName))
	}
	if login == "" {
		return nil
	}
	if displayName == "" {
		displayName = login
	}
	thumbnail := deref(stream.PreviewImageURL)
	if thumbnail == "" {
		thumbnail = thumbnailFor(login)
	}
	live := platform.LiveStatusLive
	item := platform.Item{
		ID:         "twitch:" + login,
		SourceID:   sourceID,
		Kind:       platform.KindLive,
		Title:      firstNonEmpty(deref(stream.Title), displayName),
		FetchedAt:  fetchedAt,
		URL:        platform.Str("https://www.twitch.tv/" + login),
		Thumbnail:  platform.Str(thumbnail),
		Author:     &platform.Author{Name: displayName},
		Platform:   "twitch",
		RoomID:     platform.Str(login),
		LiveStatus: &live,
		Online:     platform.Int(intOrZero(stream.ViewersCount)),
		ShowTime:   showTimeOrNil(stream.CreatedAt),
	}
	return &item
}

// categoryItem 分类榜条目 → Category(只作为「点开订阅」的目录入口)。
func categoryItem(game gqlCategoryNode, fetchedAt int64) platform.Item {
	slug := deref(game.Slug)
	title := deref(game.Name)
	if title == "" {
		title = slug
	}
	item := platform.Item{
		ID:              "twitch-category:" + slug,
		SourceID:        "twitch:categories",
		Kind:            platform.KindCategory,
		Title:           title,
		FetchedAt:       fetchedAt,
		URL:             platform.Str("https://www.twitch.tv/directory/category/" + slug),
		Thumbnail:       game.BoxArtURL,
		CategoryChannel: platform.Str("twitch:category"),
		CategoryInfo:    map[string]string{"category": slug},
	}
	if count := intOrZero(game.ViewersCount); count != 0 {
		item.Summary = platform.Str(fmt.Sprintf("%d 人在看", count))
	}
	return item
}

// searchItem 频道搜索命中 → Video(候选行信息进 summary,raw 不穿 XML)。
func searchItem(hit twitchChannelHit, fetchedAt int64) platform.Item {
	parts := make([]string, 0, 2)
	if hit.Followers != 0 {
		parts = append(parts, fmtCount(hit.Followers)+" 关注者")
	}
	if desc := truncateJS(hit.Description, 60); desc != "" {
		parts = append(parts, desc)
	}
	item := platform.Item{
		ID:        "tw-" + hit.Login,
		SourceID:  "twitch:user:search",
		Kind:      platform.KindVideo,
		Title:     hit.DisplayName,
		FetchedAt: fetchedAt,
		URL:       platform.Str("https://www.twitch.tv/" + hit.Login),
	}
	if len(parts) > 0 {
		item.Summary = platform.Str(strings.Join(parts, " · "))
	}
	return item
}

// fmtCount 关注数缩写(与桌面候选行同一读数):1.2万 / 3.4亿。
func fmtCount(n int) string {
	if n >= 1e8 {
		return strings.TrimSuffix(toFixed1(float64(n)/1e8), ".0") + "亿"
	}
	if n >= 1e4 {
		return strings.TrimSuffix(toFixed1(float64(n)/1e4), ".0") + "万"
	}
	return strconv.Itoa(n)
}

// toFixed1 对齐 JS Number.prototype.toFixed(1):对 double 精确值做十进制
// 四舍五入、平局进位(12500 → "1.3"),而非 Go 的 ties-to-even。
func toFixed1(value float64) string {
	rat := new(big.Rat).SetFloat64(value)
	if rat == nil {
		return "0.0"
	}
	scaled := rat.Mul(rat, big.NewRat(10, 1))
	quotient, remainder := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(scaled.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	digits := quotient.String()
	if len(digits) <= 1 {
		digits = "0" + digits
	}
	return digits[:len(digits)-1] + "." + digits[len(digits)-1:]
}

// truncateJS 按 UTF-16 码元截断并加省略号(对齐 JS String.slice(0, n))。
func truncateJS(s string, limit int) string {
	units := utf16.Encode([]rune(s))
	if len(units) <= limit {
		return s
	}
	return string(utf16.Decode(units[:limit])) + "…"
}

func showTimeOrNil(raw *string) *string {
	if raw == nil || *raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *raw)
	if err != nil {
		return nil
	}
	return platform.Str(t.UTC().Format("2006-01-02T15:04:05.000Z"))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
