// huya search op:live=搜房间(searchRooms,页码翻页),channel=搜主播
// (searchAnchors,rows 服务端封顶 50、start 语义怪异故不接翻页)。

package huya

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
			items, err := searchAnchors(ctx, sc, p.Keyword, p.PageSize)
			return platform.FetchPage{Items: items}, err
		},
	})
}
