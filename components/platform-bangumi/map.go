// 响应 → 契约映射(纯函数,fixture 测试覆盖):mapSubject 做条目详情,
// mapSearch 搜索端点不区分 tv/movie 统一按 tv,mapEpisodes 过滤 0 与小数
// 集号并按集号稳定排序。

package bangumi

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

const (
	subjectTypeBook  = 1
	subjectTypeAnime = 2
	subjectTypeMusic = 3
	subjectTypeGame  = 4
	subjectTypeReal  = 6
)

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// mapSubject 条目详情 → MediaMetadata(书籍/音乐与未知类型直接报错)。
func mapSubject(raw platform.RawJSON, requestedID string, mediaType platform.MediaType) (*platform.MediaMetadata, error) {
	var s rawSubject
	if err := platform.UnmarshalJSON(raw, &s); err != nil {
		return nil, err
	}
	subjectType := 0
	if s.Type != nil {
		subjectType = *s.Type
	}
	if err := checkSubjectType(subjectType, requestedID); err != nil {
		return nil, err
	}
	title, originalTitle := pickTitle(&s)
	if title == "" {
		return nil, &platform.ProviderError{
			Source:  "bangumi",
			Message: fmt.Sprintf("条目 %s 缺少标题字段", requestedID),
		}
	}
	m := platform.NewMediaMetadata(mediaType, title)
	m.OriginalTitle = originalTitle
	applySubjectFields(m, &s, requestedID)
	return m, nil
}

func checkSubjectType(subjectType int, requestedID string) error {
	if subjectType == subjectTypeBook || subjectType == subjectTypeMusic {
		kind := "音乐"
		if subjectType == subjectTypeBook {
			kind = "书籍"
		}
		return &platform.ProviderError{
			Source: "bangumi",
			Message: fmt.Sprintf(
				"条目 %s 是%s（type=%d），不是影视作品，无法生成 NFO。请确认 ID 是否正确。",
				requestedID, kind, subjectType),
		}
	}
	switch subjectType {
	case subjectTypeAnime, subjectTypeGame, subjectTypeReal, 0:
		return nil
	}
	return &platform.ProviderError{
		Source:  "bangumi",
		Message: fmt.Sprintf("条目 %s 的类型 %d 暂不支持", requestedID, subjectType),
	}
}

func applySubjectFields(m *platform.MediaMetadata, s *rawSubject, requestedID string) {
	if plot := deref(s.Summary); plot != "" {
		m.Plot = platform.Str(plot)
	}
	if date := deref(s.Date); isIsoDate(date) {
		m.Premiered = platform.Str(date)
	}
	m.UniqueIds = append(m.UniqueIds, platform.UniqueId{Source: "bangumi", Value: subjectIDValue(s, requestedID)})
	if s.Images != nil {
		if poster := deref(s.Images.Large); poster != "" {
			m.PosterURL = platform.Str(poster)
		}
	}
	m.Studios = infoboxValues(s.Infobox, []string{"动画制作", "動畫製作", "制作公司"})
	if s.Rating != nil && s.Rating.Score != nil && *s.Rating.Score > 0 {
		rating := platform.Rating{Source: "bangumi", Value: *s.Rating.Score}
		if s.Rating.Total != nil && *s.Rating.Total > 0 {
			rating.Votes = platform.Int(*s.Rating.Total)
		}
		m.Ratings = append(m.Ratings, rating)
	}
	if eps := totalEpisodes(s); eps != nil && *eps > 0 {
		m.EpisodeCount = platform.Int(*eps)
		m.SeasonCount = platform.Int(1)
	}
}

func subjectIDValue(s *rawSubject, fallback string) string {
	if s.ID != nil {
		return strconv.Itoa(*s.ID)
	}
	return fallback
}

// totalEpisodes total_episodes 优先(0 也视为已给出),否则 eps。
func totalEpisodes(s *rawSubject) *int {
	if s.TotalEpisodes != nil {
		return s.TotalEpisodes
	}
	return s.Eps
}

// mapSearch 搜索响应 → SearchResult[](limit 在首次入榜后判定,与 TS 一致)。
func mapSearch(raw platform.RawJSON, limit int) ([]platform.SearchResult, error) {
	var list rawSubjectList
	if err := platform.UnmarshalJSON(raw, &list); err != nil {
		return nil, err
	}
	out := make([]platform.SearchResult, 0, len(list.Data))
	for i := range list.Data {
		result := mapSearchItem(&list.Data[i])
		if result.Title == "" {
			continue
		}
		out = append(out, result)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func mapSearchItem(s *rawSubject) platform.SearchResult {
	title, originalTitle := pickTitle(s)
	result := platform.SearchResult{
		Source:        "bangumi",
		ID:            subjectIDValue(s, ""),
		Title:         title,
		OriginalTitle: originalTitle,
		MediaType:     platform.MediaTV,
	}
	if year := fourDigitYear(deref(s.Date)); year != "" {
		result.Year = platform.Str(year)
	}
	if s.Images != nil {
		result.PosterURL = nonEmpty(deref(s.Images.Large))
	}
	if s.Rating != nil {
		if s.Rating.Score != nil && *s.Rating.Score > 0 {
			result.Rating = platform.Float64(*s.Rating.Score)
		}
		if s.Rating.Total != nil && *s.Rating.Total > 0 {
			result.RatingVotes = platform.Int(*s.Rating.Total)
		}
	}
	result.Plot = nonEmpty(deref(s.Summary))
	if eps := totalEpisodes(s); eps != nil && *eps > 0 {
		result.EpisodeCount = platform.Int(*eps)
	}
	return result
}

func fourDigitYear(date string) string {
	y := date
	if len(y) > 4 {
		y = y[:4]
	}
	if len(y) != 4 {
		return ""
	}
	for _, c := range y {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return y
}

// mapEpisodes 每集一次请求的全集响应 → EpisodeMetadata[]。
func mapEpisodes(raw platform.RawJSON, subjectKey string) ([]platform.EpisodeMetadata, error) {
	var list rawEpisodeList
	if err := platform.UnmarshalJSON(raw, &list); err != nil {
		return nil, err
	}
	out := make([]platform.EpisodeMetadata, 0, len(list.Data))
	for i := range list.Data {
		if ep, ok := mapEpisode(&list.Data[i], subjectKey); ok {
			out = append(out, ep)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Episode < out[j].Episode })
	return out, nil
}

func mapEpisode(e *rawEpisode, subjectKey string) (platform.EpisodeMetadata, bool) {
	num, ok := episodeNumber(e)
	if !ok {
		return platform.EpisodeMetadata{}, false
	}
	nameCn := deref(e.NameCn)
	name := deref(e.Name)
	title := nameCn
	if title == "" {
		title = name
	}
	if title == "" {
		title = "第 " + strconv.Itoa(num) + " 集"
	}
	ep := platform.EpisodeMetadata{
		Season:    1,
		Episode:   num,
		Title:     title,
		Actors:    []platform.Actor{},
		UniqueIds: []platform.UniqueId{{Source: "bangumi", Value: subjectKey}},
	}
	if name != "" && name != title {
		ep.OriginalTitle = platform.Str(name)
	}
	ep.Plot = nonEmpty(deref(e.Desc))
	if aired := deref(e.Airdate); isIsoDate(aired) {
		ep.Aired = platform.Str(aired)
	}
	if e.DurationSeconds != nil && *e.DurationSeconds > 0 {
		ep.RuntimeMinutes = platform.Int(int(math.Ceil(*e.DurationSeconds / 60)))
	}
	if e.ID != nil && *e.ID > 0 {
		ep.UniqueIds = append(ep.UniqueIds, platform.UniqueId{Source: "bangumi-episode", Value: strconv.Itoa(*e.ID)})
	}
	return ep, true
}

// episodeNumber ep 优先(0 也视为已给出),否则 sort;只接受正整数集号。
func episodeNumber(e *rawEpisode) (int, bool) {
	num := e.Ep
	if num == nil {
		num = e.Sort
	}
	if num == nil || *num <= 0 || *num != math.Trunc(*num) {
		return 0, false
	}
	return int(*num), true
}
