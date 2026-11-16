// bili:square —— B站热搜（social）。条目是热搜词条，kind 必须 social：
// 声明成 article 会让词条混进阅读流（与 weibo:hot / douyin:trend 同语义）。
// wbi 签名后 GET search/square；登录 cookie 经 credentials provider 兜底。

package bili

import (
	"context"

	"mygo-host/platform"
)

func squareChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:square",
		Name: "B站热搜",
		Kind: platform.KindSocial,
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			signed, err := c.signWeb(ctx, "limit=50&platform=web", cookiePtr(info))
			if err != nil {
				return platform.PageItems{}, err
			}
			url := apiMain + "/x/web-interface/wbi/search/square?" + signed
			_, data, err := c.getJSON(ctx, url, reqOptions{Cookie: cookiePtr(info)})
			if err != nil {
				return platform.PageItems{}, err
			}
			list := jlist(jobj(jobj(data["data"])["trending"])["list"])
			return platform.PageItems{Items: squareItems(list, c.now())}, nil
		},
	})
}

// squareItems 热搜原始条目 → Social 条目。id 只用词（位次变化不算新条目）；
// content 是纯文本（social 卡片按文本渲染）。
func squareItems(list []any, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		entry := jobj(raw)
		if entry == nil {
			continue
		}
		keyword := jstr(entry["keyword"])
		link := ""
		if entry["link"] != nil {
			link = jstr(entry["link"])
		} else {
			link = jstr(entry["goto"])
		}
		if link == "" {
			query := encodePairs([][2]string{{"keyword", keyword}})
			link = "https://search.bilibili.com/all?" + query + "&from_source=webtop_search"
		}
		it := platform.Item{
			ID:        "bili-rank-" + keyword,
			SourceID:  "bili:square",
			Kind:      platform.KindSocial,
			Title:     keyword,
			URL:       platform.Str(link),
			Content:   platform.Str(keyword),
			FetchedAt: t,
			Raw:       rawJSON(entry),
		}
		if icon, ok := entry["icon"].(string); ok {
			it.Thumbnail = platform.Str(icon)
		}
		items = append(items, it)
	}
	return items
}
