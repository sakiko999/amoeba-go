// bili search op:live=搜直播间(searchBiliRooms),entity=搜主播
// (searchBiliAnchors)。与渠道形态(bili:anchor / bili:user)并存:搜索不进订阅语义。

package bili

import (
	"context"

	"mygo-host/platform"
)

func searchOps(c *client) platform.Ops {
	return platform.SearchOps(platform.SearchSpec{
		Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
		Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
			if p.Scope == platform.SearchLive {
				items, err := c.searchBiliRooms(ctx, p.Keyword, p.Page, p.PageSize)
				return platform.FetchPage{Items: items}, err
			}
			items, err := c.searchBiliAnchors(ctx, p.Keyword, p.Page)
			return platform.FetchPage{Items: items}, err
		},
	})
}
