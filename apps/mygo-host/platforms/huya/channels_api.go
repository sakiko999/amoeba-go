// 单端点同形渠道(热门/分区直播/搜主播)用声明式 MakeChannel 装配;
// 直播列表 GET 收敛在此(热门与分区共用,sourceId 按是否带 gameId 区分)。

package huya

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"mygo-host/platform"
)

// channels 渠道清单(顺序与 TS impl/register.ts 一致)。
func channels(sc *platform.ServiceContext) []platform.DataChannel {
	return []platform.DataChannel{
		newLiveChannel(sc),
		hotChannel(sc),
		categoryLiveChannel(sc),
		newCategoriesChannel(sc),
		anchorChannel(sc),
	}
}

func hotChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:         "live:huya:hot",
		Name:        "虎牙直播热门",
		Kind:        platform.KindLive,
		DefaultInfo: platform.SourceInfo{},
		Pageable:    &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, _ platform.SourceInfo, page int) (platform.PageItems, error) {
			items, err := fetchLiveList(ctx, sc, page, "")
			return platform.PageItems{Items: items}, err
		},
	})
}

func categoryLiveChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "live:huya:category",
		Name: "虎牙分区直播",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{{
			Key:      "gameId",
			Label:    "分区 ID(如 1=英雄联盟、862=CS2)",
			Required: platform.Bool(true),
		}},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			gameID := strings.TrimSpace(info["gameId"])
			if gameID == "" {
				return platform.PageItems{}, fmt.Errorf("huya:category 需要 gameId 参数")
			}
			items, err := fetchLiveList(ctx, sc, page, gameID)
			return platform.PageItems{Items: items}, err
		},
	})
}

// anchorPageSize 搜主播单次取数上限(rows 服务端封顶 50)。
const anchorPageSize = 50

func anchorChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "live:huya:anchor",
		Name: "虎牙搜主播",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{{
			Key:      "keyword",
			Label:    "主播关键词",
			Required: platform.Bool(true),
		}},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, _ int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["keyword"])
			if keyword == "" {
				return platform.PageItems{}, fmt.Errorf("huya:anchor 需要 keyword 参数")
			}
			items, err := searchAnchors(ctx, sc, keyword, anchorPageSize)
			return platform.PageItems{Items: items}, err
		},
	})
}

// fetchLiveList 直播列表一页(gameID 为空 = 热门,否则为分区)。
func fetchLiveList(ctx context.Context, sc *platform.ServiceContext, page int, gameID string) ([]platform.Item, error) {
	endpoint := fmt.Sprintf("%s&page=%d", liveListAPI, page)
	headers := map[string]string{"user-agent": uaDesktop}
	sourceID := "live:huya:hot"
	if gameID != "" {
		endpoint = fmt.Sprintf("%s&gameId=%s&page=%d", liveListAPI, url.QueryEscape(gameID), page)
		headers["referer"] = "https://www.huya.com/"
		sourceID = "live:huya:category"
	}
	raw, err := sc.HTTP.GetJSON(ctx, endpoint, headers)
	if err != nil {
		return nil, err
	}
	var res liveListResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	var rows []liveRow
	if res.Data != nil {
		rows = res.Data.Datas
	}
	t := sc.NowMs()
	items := make([]platform.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapLiveRow(row, sourceID, t))
	}
	return items, nil
}
