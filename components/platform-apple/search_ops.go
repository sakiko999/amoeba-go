// apple 聚合搜索 op:entity=播客节目,结果 URL 用 Apple 节目页。

package apple

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

func SearchOps(sc *platform.ServiceContext) platform.Ops {
	c := &catalog{sc: sc}
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchEntity},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			limit := clampInt(p.PageSize, 1, 100)
			podcasts, err := c.search(ctx, p.Keyword, limit, "cn")
			if err != nil {
				return platform.FetchPage{}, err
			}
			items, err := entityItems(podcasts, p.Keyword, sc.NowMs())
			return platform.FetchPage{Items: items}, err
		},
	})
}

func entityItems(podcasts []podcast, keyword string, fetchedAt int64) ([]platform.Item, error) {
	items := make([]platform.Item, 0, len(podcasts))
	for _, p := range podcasts {
		if p.PageURL == nil || *p.PageURL == "" {
			continue
		}
		raw, err := entityRawJSON(p)
		if err != nil {
			return nil, err
		}
		items = append(items, platform.Item{
			ID:        fmt.Sprintf("apple-podcast-%d", p.ID),
			SourceID:  searchChannelKey,
			Kind:      platform.KindAudio,
			Title:     p.Title,
			FetchedAt: fetchedAt,
			URL:       p.PageURL,
			Thumbnail: p.Artwork,
			Summary:   platform.Str(podcastSummary(p)),
			Artist:    platform.StrOmit(p.Author),
			Raw:       raw,
		})
		if p.Author != "" {
			items[len(items)-1].Author = &platform.Author{Name: p.Author}
		}
	}
	return items, nil
}
