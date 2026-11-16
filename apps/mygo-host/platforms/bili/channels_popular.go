// bili:popular —— B站综合热门（video，页码翻页）。
// hasMore 直接透上游 no_more（热门榜跨快照翻页必有重叠，不能靠本页 0 条判到底）。

package bili

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

// popularPayload /x/web-interface/popular 响应（只取用到的字段）。
type popularPayload struct {
	Data *struct {
		List   []videoRaw `json:"list"`
		NoMore *bool      `json:"no_more"`
	} `json:"data"`
}

// parsePopularItems 热门列表 payload → Item[]（纯映射，不发请求）。
func parsePopularItems(payload *popularPayload, t int64) []platform.Item {
	if payload == nil || payload.Data == nil {
		return []platform.Item{}
	}
	items := make([]platform.Item, 0, len(payload.Data.List))
	for _, raw := range payload.Data.List {
		items = append(items, ugc("bili:popular", t, videoInput(raw)))
	}
	return items
}

func popularChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:popular",
		Name:     "B站综合热门",
		Kind:     platform.KindVideo,
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			url := fmt.Sprintf("%s/x/web-interface/popular?pn=%d&ps=20", apiMain, page)
			raw, _, err := c.getJSON(ctx, url, reqOptions{Cookie: cookiePtr(info)})
			if err != nil {
				return platform.PageItems{}, err
			}
			var payload popularPayload
			if err := platform.UnmarshalJSON(raw, &payload); err != nil {
				return platform.PageItems{}, err
			}
			items := parsePopularItems(&payload, c.now())
			hasMore := true
			if payload.Data != nil && payload.Data.NoMore != nil {
				hasMore = !*payload.Data.NoMore
			}
			return platform.PageItems{Items: items, HasMore: platform.Bool(hasMore)}, nil
		},
	})
}
