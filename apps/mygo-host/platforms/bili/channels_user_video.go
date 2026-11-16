// bili:user_video —— B站 UP 主投稿（video，页码翻页，30 条/页）。
// space 系 WAF 要求 dm_* 指纹（signSpace）且必须带登录 cookie；
// hasMore 优先用 data.page.count 真实总数，缺失时退回「满页即可能有下一页」。

package bili

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

const userVideoPageSize = 30

func userVideoChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:user_video",
		Name:     "B站 UP 主投稿",
		Kind:     platform.KindVideo,
		InfoTpl:  []platform.SourceInfoField{requiredField("uid", "UP 主 uid")},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			uid := info["uid"]
			if uid == "" {
				return platform.PageItems{}, fmt.Errorf("bili:user_video 需要 uid")
			}
			cookie := nonEmptyCookie(info)
			query := fmt.Sprintf("mid=%s&ps=%d&pn=%d&platform=web&order=pubdate&keyword=&tid=0&web_location=1550101",
				uid, userVideoPageSize, page)
			signed, err := c.signSpace(ctx, query, cookie)
			if err != nil {
				return platform.PageItems{}, err
			}
			url := apiMain + "/x/space/wbi/arc/search?" + signed
			raw, _, err := c.getJSON(ctx, url, reqOptions{
				Referer: "https://space.bilibili.com/" + uid + "/video",
				Cookie:  cookie,
				Buvid:   true,
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			var resp struct {
				Data *struct {
					List *struct {
						Vlist []videoRaw `json:"vlist"`
					} `json:"list"`
					Page *struct {
						Count *float64 `json:"count"`
					} `json:"page"`
				} `json:"data"`
			}
			if err := platform.UnmarshalJSON(raw, &resp); err != nil {
				return platform.PageItems{}, err
			}
			var vlist []videoRaw
			var count *float64
			if resp.Data != nil {
				if resp.Data.List != nil {
					vlist = resp.Data.List.Vlist
				}
				if resp.Data.Page != nil {
					count = resp.Data.Page.Count
				}
			}
			t := c.now()
			items := make([]platform.Item, 0, len(vlist))
			for _, v := range vlist {
				in := videoInput(v)
				in.Desc = deref(v.Description)
				in.Pubdate = int64(jnumPtr(v.Created))
				// 该接口不返回时长；充电专属只体现在 is_charging_arc。
				in.Duration = nil
				in.Charging = v.IsChargingArc
				in.Owner = ""
				if v.Author != nil && *v.Author != "" {
					in.Owner = *v.Author
				}
				items = append(items, ugc("bili:user_video", t, in))
			}
			pageItems := platform.PageItems{Items: items}
			if count != nil {
				pageItems.HasMore = platform.Bool(float64(page*userVideoPageSize) < *count)
				pageItems.Total = platform.Int(int(*count))
			} else {
				pageItems.HasMore = platform.Bool(len(vlist) >= userVideoPageSize)
			}
			return pageItems, nil
		},
	})
}
