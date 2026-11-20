// youtube search op:live=搜正在直播(searchYouTubeLive,保持搜索相关性
// 不做观看数排序),entity=搜频道(searchYouTubeChannels)。

package youtube

import (
	"context"

	"mygo-host/platform"
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			if p.Scope == platform.SearchLive {
				hits, err := searchYouTubeLive(ctx, sc, p.Keyword)
				if err != nil {
					return platform.FetchPage{}, err
				}
				now := sc.NowMs()
				items := make([]platform.Item, 0, len(hits))
				for _, hit := range hits {
					items = append(items, youtubeLiveHitToLive("youtube:search-live", hit, now))
				}
				return platform.FetchPage{Items: items}, nil
			}
			hits, err := searchYouTubeChannels(ctx, sc, p.Keyword)
			if err != nil {
				return platform.FetchPage{}, err
			}
			return platform.FetchPage{Items: channelHitsToItems("youtube:user", hits, sc.NowMs())}, nil
		},
	})
}
