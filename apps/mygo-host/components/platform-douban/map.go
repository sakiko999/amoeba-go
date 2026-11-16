// 上游条目 → 契约元数据映射(纯函数,fixture 测试覆盖;对齐 provider.ts 的 mapSubject)。

package douban

import (
	"fmt"
	"regexp"
	"strings"

	"mygo-host/platform"
)

var premieredRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// mapSubject 映射一个豆瓣条目;缺标题报错(TS 侧同为普通 Error)。
func mapSubject(raw platform.RawJSON, requestedID string) (*platform.MediaMetadata, error) {
	var s doubanSubject
	if err := platform.UnmarshalJSON(raw, &s); err != nil {
		return nil, err
	}
	title := trimDeref(s.Title)
	if title == "" {
		return nil, fmt.Errorf("[douban] 条目 %s 缺少标题", requestedID)
	}
	isTv := isTvSubject(&s)
	m := platform.NewMediaMetadata(mediaTypeOf(isTv), title)
	m.OriginalTitle = originalTitleOf(&s, title)
	m.Plot = strOrNil(trimDeref(s.Intro))
	if date := trimDeref(s.ReleaseDate); premieredRe.MatchString(date) {
		m.Premiered = platform.Str(date)
	}
	m.UniqueIds = append(m.UniqueIds, platform.UniqueId{Source: "douban", Value: subjectID(&s, requestedID)})
	m.Genres = mapGenres(s.Genres)
	if rating, ok := mapRating(s.Rating); ok {
		m.Ratings = append(m.Ratings, rating)
	}
	m.PosterURL = posterOf(&s)
	m.Actors = mapCast(s.Directors, s.Actors)
	if isTv && s.EpisodesCount != nil && *s.EpisodesCount > 0 {
		m.EpisodeCount = platform.Int(*s.EpisodesCount)
		m.SeasonCount = platform.Int(1)
	}
	return m, nil
}

// isTvSubject is_tv 显式 true,或带正集数(电视条目可能缺 is_tv)。
func isTvSubject(s *doubanSubject) bool {
	return (s.IsTv != nil && *s.IsTv) || (s.EpisodesCount != nil && *s.EpisodesCount > 0)
}

func mediaTypeOf(isTv bool) platform.MediaType {
	if isTv {
		return platform.MediaTV
	}
	return platform.MediaMovie
}

// subjectID 上游 id 优先(含空串),缺失时回退调用方给的 id。
func subjectID(s *doubanSubject, fallback string) string {
	if s.ID != nil {
		return *s.ID
	}
	return fallback
}

func originalTitleOf(s *doubanSubject, title string) *string {
	original := trimDeref(s.OriginalTitle)
	if original == "" || original == title {
		return nil
	}
	return platform.Str(original)
}

func strOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return platform.Str(v)
}

func mapGenres(genres []string) []string {
	out := make([]string, 0, len(genres))
	for _, g := range genres {
		if trimmed := strings.TrimSpace(g); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// mapRating 0 分视为无评分;票数 0 视为缺失。
func mapRating(r *doubanRating) (platform.Rating, bool) {
	if r == nil || r.Value == nil || *r.Value <= 0 {
		return platform.Rating{}, false
	}
	rating := platform.Rating{Source: "douban", Value: *r.Value}
	if r.Count != nil && *r.Count > 0 {
		rating.Votes = platform.Int(*r.Count)
	}
	return rating, true
}

// posterOf cover_url 存在即用(含空串,不回落 pic),否则取 pic.large。
func posterOf(s *doubanSubject) *string {
	var poster string
	if s.CoverURL != nil {
		poster = *s.CoverURL
	} else if s.Pic != nil && s.Pic.Large != nil {
		poster = *s.Pic.Large
	}
	poster = strings.TrimSpace(poster)
	if poster == "" {
		return nil
	}
	return platform.Str(poster)
}

// mapCast 导演在前(role=导演)、演员在后;order 取各自原始下标,导演占去演员的偏移。
func mapCast(directors, actors []doubanPerson) []platform.Actor {
	cast := make([]platform.Actor, 0, len(directors)+len(actors))
	for i, p := range directors {
		if name := actorName(p); name != "" {
			cast = append(cast, platform.Actor{Name: name, Role: platform.Str("导演"), Order: i})
		}
	}
	for i, p := range actors {
		if name := actorName(p); name != "" {
			cast = append(cast, platform.Actor{Name: name, Order: i + len(directors)})
		}
	}
	return cast
}

// actorName trim 后过滤空名与上游字面量 "undefined"。
func actorName(p doubanPerson) string {
	name := trimDeref(p.Name)
	if name == "undefined" {
		return ""
	}
	return name
}

func trimDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
