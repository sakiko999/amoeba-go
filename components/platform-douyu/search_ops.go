// douyu search op:live=搜房间(searchRooms),channel=搜主播(searchAnchors)。
// 两个端点均匿名可用;与渠道形态(live:douyu:anchor)并存。

package douyu

import (
	"context"

	"mygo-host/platform"
)

func searchOps(sc *platform.ServiceContext) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchChannel},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			if p.Scope == platform.SearchLive {
				items, err := searchRooms(ctx, sc, p.Keyword, p.Page, p.PageSize)
				return platform.FetchPage{Items: items}, err
			}
			items, err := searchAnchors(ctx, sc, p.Keyword, p.Page, p.PageSize)
			return platform.FetchPage{Items: items}, err
		},
	})
}
