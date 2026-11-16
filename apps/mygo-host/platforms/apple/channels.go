// apple 数据渠道:热门榜与关键词搜索。两者同构(kind=audio,url=节目 RSS feed),
// 均无翻页与被动采集;key / 参数表 / 缺省参数与 TS makeApiChannel 声明一致。

package apple

import (
	"context"
	"fmt"
	"strings"

	"mygo-host/platform"
)

const (
	chartChannelKey  = "apple:podcast-chart"
	searchChannelKey = "apple:podcast-search"
)

var podcastGenres = []struct {
	id    int
	label string
}{
	{26, "全部"},
	{1324, "社会与文化"},
	{1304, "教育"},
	{1321, "商业"},
	{1318, "科技"},
	{1303, "喜剧"},
	{1489, "新闻"},
	{1512, "健康与健身"},
}

func channels(sc *platform.ServiceContext) []platform.DataChannel {
	c := &catalog{sc: sc}
	return []platform.DataChannel{
		chartChannel(c, sc),
		searchChannel(c, sc),
	}
}

func chartChannel(c *catalog, sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  chartChannelKey,
		Name: "Apple 播客热门榜",
		Kind: platform.KindAudio,
		InfoTpl: []platform.SourceInfoField{
			{Key: "genre", Label: "分类(" + genreOptions() + ")", Required: platform.Bool(false)},
			{Key: "country", Label: "国家区(如 cn / us),默认 cn", Required: platform.Bool(false)},
			{Key: "limit", Label: "条数(默认 20,最多 200)", Required: platform.Bool(false)},
		},
		DefaultInfo: platform.SourceInfo{"genre": "26"},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, _ int) (platform.PageItems, error) {
			genre := paramNumber(info, "genre", 26)
			limit := clampInt(paramNumber(info, "limit", 20), 1, 200)
			podcasts, err := c.chart(ctx, genre, limit, paramCountry(info))
			if err != nil {
				return platform.PageItems{}, err
			}
			items, err := podcastItems(podcasts, chartChannelKey, true, sc.NowMs())
			return platform.PageItems{Items: items}, err
		},
	})
}

func searchChannel(c *catalog, sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  searchChannelKey,
		Name: "Apple 播客搜索",
		Kind: platform.KindAudio,
		InfoTpl: []platform.SourceInfoField{
			{Key: "q", Label: "关键词", Required: platform.Bool(true)},
			{Key: "country", Label: "国家区(如 cn / us),默认 cn", Required: platform.Bool(false)},
			{Key: "limit", Label: "条数(默认 20,最多 100)", Required: platform.Bool(false)},
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, _ int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["q"])
			if keyword == "" {
				return platform.PageItems{}, fmt.Errorf("apple:podcast-search 需要 q 参数")
			}
			limit := clampInt(paramNumber(info, "limit", 20), 1, 100)
			podcasts, err := c.search(ctx, keyword, limit, paramCountry(info))
			if err != nil {
				return platform.PageItems{}, err
			}
			items, err := podcastItems(podcasts, searchChannelKey, false, sc.NowMs())
			return platform.PageItems{Items: items}, err
		},
	})
}

func genreOptions() string {
	parts := make([]string, 0, len(podcastGenres))
	for _, genre := range podcastGenres {
		parts = append(parts, fmt.Sprintf("%d=%s", genre.id, genre.label))
	}
	return strings.Join(parts, " / ")
}
