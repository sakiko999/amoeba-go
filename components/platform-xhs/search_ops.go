// xhs search op:仅 entity(搜用户)。page/pageSize 由 SearchOps 归一,
// 搜索页 SSR 投影不分页,这里接收后不转发。

package xhs

import (
	"context"

	"mygo-host/platform"
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchEntity},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			items, err := searchUsers(ctx, sc, p.Keyword)
			if err != nil {
				return platform.FetchPage{}, err
			}
			return platform.FetchPage{Items: items}, nil
		},
	})
}
