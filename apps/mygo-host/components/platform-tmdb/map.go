// 响应 → 契约映射(纯函数,fixture 测试覆盖;与 provider.ts 的 mapXxx 对齐)。

package tmdb

import (
	"fmt"
	"sort"
	"strings"

	"mygo-host/platform"
)

func posterURL(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return platform.Str(img + "/w500" + *p)
}

func thumbURL(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return platform.Str(img + "/w300" + *p)
}

func mapCast(cast []person) []platform.Actor {
	if len(cast) > 15 {
		cast = cast[:15]
	}
	actors := make([]platform.Actor, 0, len(cast))
	for i, c := range cast {
		order := i
		if c.Order != nil {
			order = *c.Order
		}
		actor := platform.Actor{Name: c.Name, Role: platform.StrOmit(c.Character), Order: order}
		if c.ProfilePath != nil {
			actor.ThumbURL = platform.Str(img + "/w185" + *c.ProfilePath)
		}
		actors = append(actors, actor)
	}
	return actors
}

func mapNames(list []nameOnly) []string {
	names := make([]string, 0, len(list))
	for _, n := range list {
		if n.Name != "" {
			names = append(names, n.Name)
		}
	}
	return names
}

func mapRating(voteAverage *float64, voteCount *int) (platform.Rating, bool) {
	if voteAverage == nil || *voteAverage <= 0 {
		return platform.Rating{}, false
	}
	rating := platform.Rating{Source: "themoviedb", Value: *voteAverage}
	if voteCount != nil && *voteCount > 0 {
		rating.Votes = platform.Int(*voteCount)
	}
	return rating, true
}

func mapUniqueIDs(id *int, fallback string, ext *externalIDs) []platform.UniqueId {
	value := fallback
	if id != nil {
		value = fmt.Sprintf("%d", *id)
	}
	ids := []platform.UniqueId{{Source: "tmdb", Value: value}}
	if ext != nil && ext.ImdbID != nil && *ext.ImdbID != "" {
		ids = append(ids, platform.UniqueId{Source: "imdb", Value: *ext.ImdbID})
	}
	return ids
}

// mapTvDetail 电视详情 → MediaMetadata。
func mapTvDetail(raw platform.RawJSON, id string) (*platform.MediaMetadata, error) {
	var tv tvDetail
	if err := platform.UnmarshalJSON(raw, &tv); err != nil {
		return nil, err
	}
	name := deref(tv.Name)
	original := deref(tv.OriginalName)
	title := firstNonEmpty(name, original)
	if title == "" {
		return nil, &platform.ProviderError{Source: "tmdb", Message: fmt.Sprintf("条目 %s 缺少标题", id)}
	}
	m := platform.NewMediaMetadata(platform.MediaTV, title)
	if original != title {
		m.OriginalTitle = platform.StrOmit(original)
	}
	m.Plot = platform.StrOmit(deref(tv.Overview))
	m.Premiered = platform.StrOmit(deref(tv.FirstAirDate))
	m.Status = platform.StrOmit(deref(tv.Status))
	m.Genres = mapNames(tv.Genres)
	m.Studios = mapNames(tv.ProductionCos)
	m.UniqueIds = mapUniqueIDs(tv.ID, id, tv.ExternalIDs)
	if r, ok := mapRating(tv.VoteAverage, tv.VoteCount); ok {
		m.Ratings = append(m.Ratings, r)
	}
	if tv.Credits != nil {
		m.Actors = mapCast(tv.Credits.Cast)
	}
	m.PosterURL = posterURL(tv.PosterPath)
	m.SeasonCount = tv.NumberOfSeasons
	m.EpisodeCount = tv.NumberOfEpisodes
	return m, nil
}

// mapMovieDetail 电影详情 → MediaMetadata。
func mapMovieDetail(raw platform.RawJSON, id string) (*platform.MediaMetadata, error) {
	var mv movieDetail
	if err := platform.UnmarshalJSON(raw, &mv); err != nil {
		return nil, err
	}
	title := firstNonEmpty(deref(mv.Title), deref(mv.OriginalTitle))
	if title == "" {
		return nil, &platform.ProviderError{Source: "tmdb", Message: fmt.Sprintf("条目 %s 缺少标题", id)}
	}
	m := platform.NewMediaMetadata(platform.MediaMovie, title)
	if original := deref(mv.OriginalTitle); original != title {
		m.OriginalTitle = platform.StrOmit(original)
	}
	m.Plot = platform.StrOmit(deref(mv.Overview))
	m.Premiered = platform.StrOmit(deref(mv.ReleaseDate))
	m.Status = platform.StrOmit(deref(mv.Status))
	m.Genres = mapNames(mv.Genres)
	m.Studios = mapNames(mv.ProductionCos)
	m.UniqueIds = mapUniqueIDs(mv.ID, id, mv.ExternalIDs)
	if r, ok := mapRating(mv.VoteAverage, mv.VoteCount); ok {
		m.Ratings = append(m.Ratings, r)
	}
	if mv.Credits != nil {
		m.Actors = mapCast(mv.Credits.Cast)
	}
	m.PosterURL = posterURL(mv.PosterPath)
	return m, nil
}

func mapSearchItem(item searchItem, mediaType platform.MediaType) platform.SearchResult {
	result := platform.SearchResult{Source: "tmdb", MediaType: mediaType}
	if item.ID != nil {
		result.ID = fmt.Sprintf("%d", *item.ID)
	}
	var rawTitle, original, date string
	if mediaType == platform.MediaTV {
		rawTitle, original, date = deref(item.Name), deref(item.OriginalName), deref(item.FirstAirDate)
	} else {
		rawTitle, original, date = deref(item.Title), deref(item.OriginalTitle), deref(item.ReleaseDate)
	}
	result.Title = firstNonEmpty(rawTitle, original)
	// TS 语义:originalTitle 只在与「主标题(非回退值)」不同时给出。
	if original != rawTitle && original != "" {
		result.OriginalTitle = platform.Str(original)
	}
	if len(date) >= 4 {
		result.Year = platform.Str(date[:4])
	}
	result.PosterURL = posterURL(item.PosterPath)
	if item.VoteAverage != nil && *item.VoteAverage > 0 {
		result.Rating = platform.Float64(*item.VoteAverage)
	}
	if item.VoteCount != nil && *item.VoteCount > 0 {
		result.RatingVotes = platform.Int(*item.VoteCount)
	}
	result.Plot = platform.StrOmit(deref(item.Overview))
	return result
}

// mapSearch 搜索响应 → SearchResult[](limit 限量)。
func mapSearch(raw platform.RawJSON, mediaType platform.MediaType, limit int) ([]platform.SearchResult, error) {
	var res searchResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	if len(res.Results) > limit {
		res.Results = res.Results[:limit]
	}
	results := make([]platform.SearchResult, 0, len(res.Results))
	for _, item := range res.Results {
		results = append(results, mapSearchItem(item, mediaType))
	}
	return results, nil
}

// mapSeasonEpisodes 季详情 → EpisodeMetadata[](过滤 0 集,按集号升序)。
func mapSeasonEpisodes(raw platform.RawJSON, season int) ([]platform.EpisodeMetadata, error) {
	var detail seasonDetail
	if err := platform.UnmarshalJSON(raw, &detail); err != nil {
		return nil, err
	}
	episodes := make([]platform.EpisodeMetadata, 0, len(detail.Episodes))
	for _, e := range detail.Episodes {
		number := 0
		if e.EpisodeNumber != nil {
			number = *e.EpisodeNumber
		}
		if number <= 0 {
			continue
		}
		title := deref(e.Name)
		if title == "" {
			title = fmt.Sprintf("第 %d 集", number)
		}
		ep := platform.EpisodeMetadata{
			Season:    season,
			Episode:   number,
			Title:     title,
			Plot:      platform.StrOmit(deref(e.Overview)),
			Aired:     platform.StrOmit(deref(e.AirDate)),
			Actors:    []platform.Actor{},
			UniqueIds: []platform.UniqueId{},
			ThumbURL:  thumbURL(e.StillPath),
		}
		if e.Runtime != nil && *e.Runtime > 0 {
			ep.RuntimeMinutes = platform.Int(*e.Runtime)
		}
		episodes = append(episodes, ep)
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].Episode < episodes[j].Episode })
	return episodes, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
