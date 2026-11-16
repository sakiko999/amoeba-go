// podcast → feed Item 映射(纯函数):字段与 TS chart.ts / search.ts 的 Audio
// 对象逐一对齐;可选字段按「有值才输出」用指针 + omitempty。

package apple

import (
	"encoding/json"
	"fmt"

	"mygo-host/platform"
)

// podcastRaw 条目 raw 字段(榜单位置 / Apple 目录 id / 节目页 / feed 地址)。
type podcastRaw struct {
	Rank    *int    `json:"rank,omitempty"`
	AppleID int64   `json:"appleId"`
	PageURL *string `json:"pageUrl,omitempty"`
	FeedURL string  `json:"feedUrl"`
}

func podcastItems(podcasts []podcast, sourceID string, withRank bool, fetchedAt int64) ([]platform.Item, error) {
	items := make([]platform.Item, 0, len(podcasts))
	for index, p := range podcasts {
		item, err := podcastItem(p, sourceID, index+1, withRank, fetchedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func podcastItem(p podcast, sourceID string, rank int, withRank bool, fetchedAt int64) (platform.Item, error) {
	raw, err := podcastRawJSON(p, rank, withRank)
	if err != nil {
		return platform.Item{}, err
	}
	item := platform.Item{
		ID:        fmt.Sprintf("apple-podcast-%d", p.ID),
		SourceID:  sourceID,
		Kind:      platform.KindAudio,
		Title:     p.Title,
		FetchedAt: fetchedAt,
		URL:       platform.Str(p.FeedURL),
		Thumbnail: p.Artwork,
		Summary:   platform.Str(podcastSummary(p)),
		Artist:    platform.StrOmit(p.Author),
		Raw:       raw,
	}
	if p.Author != "" {
		item.Author = &platform.Author{Name: p.Author}
	}
	return item, nil
}

func podcastSummary(p podcast) string {
	summary := p.Author
	if p.Genre != nil && *p.Genre != "" {
		summary += " · " + *p.Genre
	}
	if p.EpisodeCount != nil && *p.EpisodeCount != 0 {
		summary += fmt.Sprintf(" · %d 集", *p.EpisodeCount)
	}
	return summary
}

func podcastRawJSON(p podcast, rank int, withRank bool) (platform.RawJSON, error) {
	raw := podcastRaw{AppleID: p.ID, PageURL: p.PageURL, FeedURL: p.FeedURL}
	if withRank {
		raw.Rank = platform.Int(rank)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return data, nil
}
