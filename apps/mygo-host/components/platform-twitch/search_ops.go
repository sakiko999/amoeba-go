// twitch search op:仅 channel(搜频道);live 搜索需 searchFor→streams 两步
// enrich,阶段 2 再做(见 docs/mygo-search-page-plan.md §2)。

package twitch

import (
	"context"

	"mygo-host/platform"
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchChannel},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
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
