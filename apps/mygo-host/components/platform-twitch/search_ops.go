// twitch search op:channel 搜频道(Video item);live 搜直播间 = 频道搜索
// 命中后一次 GQL 批量 enrich stream、筛掉未开播的(两步,方案 §2)。

package twitch

import (
	"context"

	"mygo-host/platform"
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchChannel, platform.SearchLive},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			if p.Scope == platform.SearchLive {
				return searchLivePage(ctx, sc, p.Keyword)
			}
			hits, err := searchTwitchChannels(ctx, sc, p.Keyword)
			if err != nil {
				return platform.FetchPage{}, err
			}
			now := sc.NowMs()
			items := make([]platform.Item, 0, len(hits))
			for _, hit := range hits {
				items = append(items, searchItem(hit, now))
			}
			return platform.FetchPage{Items: items}, nil
		},
	})
}

// searchLivePage 直播间搜索:searchFor 的相关性序保留,只返回在播的频道。
func searchLivePage(ctx context.Context, sc *platform.ServiceContext, keyword string) (platform.FetchPage, error) {
	hits, err := searchTwitchChannels(ctx, sc, keyword)
	if err != nil {
		return platform.FetchPage{}, err
	}
	logins := make([]string, 0, len(hits))
	for _, hit := range hits {
		logins = append(logins, hit.Login)
	}
	users, err := fetchTwitchUsers(ctx, sc, logins)
	if err != nil {
		return platform.FetchPage{}, err
	}
	now := sc.NowMs()
	items := make([]platform.Item, 0, len(users))
	for i, user := range users {
		if user == nil || user.Stream == nil {
			continue
		}
		item := twitchLiveItem(user, logins[i], now)
		item.SourceID = "twitch:live:search"
		items = append(items, item)
	}
	return platform.FetchPage{Items: items}, nil
}
