// bili 搜索渠道：搜主播（live，页码翻页）/ 搜 UP 主（video，仅取数不做长期订阅）。
// 搜索接口匿名可用，标题带 <em> 高亮标签需剥离。

package bili

import (
	"context"
	"fmt"
	"strings"

	"mygo-host/platform"
)

func anchorChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:anchor",
		Name:     "B站搜主播",
		Kind:     platform.KindLive,
		InfoTpl:  []platform.SourceInfoField{requiredField("keyword", "主播关键词")},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["keyword"])
			if keyword == "" {
				return platform.PageItems{}, fmt.Errorf("bili:anchor 需要 keyword 参数")
			}
			items, err := c.searchBiliAnchors(ctx, keyword, page)
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

func userChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:user",
		Name: "B站搜UP主",
		Kind: platform.KindVideo,
		InfoTpl: []platform.SourceInfoField{{
			Key:         "keyword",
			Label:       "UP主关键词",
			Placeholder: platform.Str("如:半佛仙人"),
			Required:    platform.Bool(true),
		}},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["keyword"])
			if keyword == "" {
				return platform.PageItems{}, fmt.Errorf("bili:user 需要 keyword 参数")
			}
			users, err := c.searchBiliUsers(ctx, keyword, 1)
			if err != nil {
				return platform.PageItems{}, err
			}
			t := c.now()
			items := make([]platform.Item, 0, len(users))
			for _, u := range users {
				items = append(items, platform.Item{
					ID:        "up-" + u.Mid,
					SourceID:  "bili:user",
					Kind:      platform.KindVideo,
					Title:     u.Uname,
					URL:       platform.Str("https://space.bilibili.com/" + u.Mid),
					Thumbnail: u.Upic,
					// 候选行次行信息：raw 不穿 XML 边界，summary 是唯一能到 UI 的自由字段。
					Summary:   platform.Str(fmt.Sprintf("%d 粉丝 · %d 个视频", intValue(u.Fans), intValue(u.Videos))),
					FetchedAt: t,
				})
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

func intValue(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
