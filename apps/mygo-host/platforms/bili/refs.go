// 合集 / 视频列表 / 收藏夹的参数归一化（纯函数）：三类 ID 来源 URL 各不相同，
// 合集还允许直接粘视频链接（调用方再联网换 seasonId）。网络请求不在这里。

package bili

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	schemeRe          = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)
	digitsRe          = regexp.MustCompile(`^\d+$`)
	videoRefRe        = regexp.MustCompile(`(?i)^(BV[0-9A-Za-z]{10}|av\d+)$`)
	seasonVideoPathRe = regexp.MustCompile(`(?i)/video/(BV[0-9A-Za-z]{10}|av\d+)`)
	seriesListPathRe  = regexp.MustCompile(`/lists/(\d+)`)
)

// biliSeasonRef 已知 seasonId，或需要再查一次视频详情的视频 ID。
type biliSeasonRef struct {
	SeasonID string
	VideoID  string
}

// biliSeriesRef 视频列表引用。
type biliSeriesRef struct {
	SeriesID string
}

// biliFavlistRef 收藏夹引用。
type biliFavlistRef struct {
	MediaID string
}

func parseBiliSeasonRef(input string) *biliSeasonRef {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil
	}
	if digitsRe.MatchString(raw) {
		return &biliSeasonRef{SeasonID: raw}
	}
	if videoRefRe.MatchString(raw) {
		return &biliSeasonRef{VideoID: raw}
	}
	u := parseBiliURL(raw)
	if u == nil {
		return nil
	}
	query := u.Query()
	if id := firstDigits(query.Get("season_id"), query.Get("sid")); id != "" {
		return &biliSeasonRef{SeasonID: id}
	}
	if m := seasonVideoPathRe.FindStringSubmatch(u.Path); len(m) == 2 {
		return &biliSeasonRef{VideoID: m[1]}
	}
	return nil
}

func parseBiliSeriesRef(input string) *biliSeriesRef {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil
	}
	if digitsRe.MatchString(raw) {
		return &biliSeriesRef{SeriesID: raw}
	}
	u := parseBiliURL(raw)
	if u == nil {
		return nil
	}
	query := u.Query()
	id := firstDigits(query.Get("sid"), query.Get("series_id"), seriesListPathRe.FindStringSubmatch(u.Path))
	if id == "" {
		return nil
	}
	return &biliSeriesRef{SeriesID: id}
}

func parseBiliFavlistRef(input string) *biliFavlistRef {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return nil
	}
	if digitsRe.MatchString(raw) {
		return &biliFavlistRef{MediaID: raw}
	}
	u := parseBiliURL(raw)
	if u == nil {
		return nil
	}
	query := u.Query()
	if id := firstDigits(query.Get("fid"), query.Get("media_id")); id != "" {
		return &biliFavlistRef{MediaID: id}
	}
	return nil
}

// parseBiliURL 补 scheme 后解析，并限定 bilibili.com 域。
func parseBiliURL(input string) *url.URL {
	raw := input
	if !schemeRe.MatchString(raw) {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com") {
		return nil
	}
	return u
}

// firstDigits 依次取第一个纯数字候选；regexp 子匹配取第 1 组。
func firstDigits(candidates ...any) string {
	for _, c := range candidates {
		switch v := c.(type) {
		case string:
			s := strings.TrimSpace(v)
			if digitsRe.MatchString(s) {
				return s
			}
		case []string:
			if len(v) >= 2 && digitsRe.MatchString(v[1]) {
				return v[1]
			}
		}
	}
	return ""
}
