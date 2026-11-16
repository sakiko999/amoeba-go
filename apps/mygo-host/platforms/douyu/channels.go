// douyu 渠道清单:对齐 TS impl/register.ts 的 5 个渠道(key/name/kind/
// sourceInfoTpl/defaultInfo/顺序完全一致)。单端点声明式渠道用 platform.MakeChannel,
// live(多房间聚合)与 categories(无翻页)用本地 simpleChannel —— TS 侧它们也没有
// fetchMore,不能让 Go 的 MakeChannel 误暴露翻页能力。

package douyu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"mygo-host/platform"
)

const anchorPageSize = 20

// simpleChannel 无翻页能力的渠道(对应 TS 只装配 fetch 的 DataSource)。
type simpleChannel struct {
	key         string
	name        string
	kind        platform.Kind
	infoTpl     []platform.SourceInfoField
	defaultInfo platform.SourceInfo
	fetch       func(ctx context.Context, info platform.SourceInfo) (platform.FetchResult, error)
}

func (c *simpleChannel) Key() string                               { return c.key }
func (c *simpleChannel) Name() string                              { return c.name }
func (c *simpleChannel) Kind() platform.Kind                       { return c.kind }
func (c *simpleChannel) SourceInfoTpl() []platform.SourceInfoField { return c.infoTpl }
func (c *simpleChannel) DefaultInfo() platform.SourceInfo          { return c.defaultInfo }

func (c *simpleChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &simpleSource{fetch: func(ctx context.Context) (platform.FetchResult, error) {
		return c.fetch(ctx, info)
	}}
}

type simpleSource struct {
	fetch func(ctx context.Context) (platform.FetchResult, error)
}

func (s *simpleSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	return s.fetch(ctx)
}

// channels 返回 douyu 全部渠道实例(顺序与 TS douyuChannels 一致)。
func channels(sc *platform.ServiceContext) []platform.DataChannel {
	return []platform.DataChannel{
		newLiveChannel(sc),
		newHotChannel(sc),
		newCategoryChannel(sc),
		newCategoriesChannel(sc),
		newAnchorChannel(sc),
	}
}

// newLiveChannel 直播房间:roomIds 逐房间并发抓取,单房间失败跳过,全部失败报错。
func newLiveChannel(sc *platform.ServiceContext) platform.DataChannel {
	return &simpleChannel{
		key:  "live:douyu",
		name: "斗鱼直播房间",
		kind: platform.KindLive,
		infoTpl: []platform.SourceInfoField{{
			Key: "roomIds", Label: "直播间 ID(逗号分隔,可多个)", Required: platform.Bool(true),
		}},
		fetch: func(ctx context.Context, info platform.SourceInfo) (platform.FetchResult, error) {
			return fetchLiveRooms(ctx, sc, info)
		},
	}
}

func fetchLiveRooms(ctx context.Context, sc *platform.ServiceContext, info platform.SourceInfo) (platform.FetchResult, error) {
	roomIDs := parseRoomIds(info)
	if len(roomIDs) == 0 {
		return platform.FetchResult{}, errors.New("live:douyu 需要 roomIds")
	}
	t := sc.NowMs()
	items := make([]*platform.Item, len(roomIDs))
	var wg sync.WaitGroup
	for i, roomID := range roomIDs {
		wg.Add(1)
		go func(i int, roomID string) {
			defer wg.Done()
			room, err := roomInfo(ctx, sc, roomID)
			if err != nil {
				sc.Log("douyu: 房间 %s 拉取失败,跳过: %v", roomID, err)
				return
			}
			item := mapRoomItem(roomID, room, t)
			items[i] = &item
		}(i, roomID)
	}
	wg.Wait()
	result := make([]platform.Item, 0, len(roomIDs))
	for _, item := range items {
		if item != nil {
			result = append(result, *item)
		}
	}
	if len(result) == 0 {
		return platform.FetchResult{}, fmt.Errorf("live:douyu 全部房间拉取失败(%d 个)", len(roomIDs))
	}
	return platform.FetchResult{Items: result}, nil
}

// newHotChannel 直播热门:页码翻页,hasMore 按过滤前原始行数(工厂契约)。
func newHotChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:         "live:douyu:hot",
		Name:        "斗鱼直播热门",
		Kind:        platform.KindLive,
		DefaultInfo: platform.SourceInfo{},
		Pageable:    &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			url := fmt.Sprintf("%s/japi/weblist/apinc/allpage/6/%d", baseURL, page)
			raw, err := sc.HTTP.GetJSON(ctx, url, map[string]string{
				"user-agent": clientUA, "referer": baseURL + "/",
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			var res hotListResponse
			if err := platform.UnmarshalJSON(raw, &res); err != nil {
				return platform.PageItems{}, err
			}
			items := mapHotItems(res.Data.Rl, sc.NowMs())
			return platform.PageItems{Items: items, HasMore: platform.Bool(len(res.Data.Rl) > 0)}, nil
		},
	})
}

// newCategoryChannel 按二级分区列房(areaId → cate2Id)。
func newCategoryChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "live:douyu:category",
		Name: "斗鱼分区直播",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{{
			Key: "areaId", Label: "分区 ID(如 1=英雄联盟、181=王者荣耀)", Required: platform.Bool(true),
		}},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			areaID := strings.TrimSpace(jsString(info["areaId"]))
			if areaID == "" {
				return platform.PageItems{}, errors.New("douyu:category 需要 areaId 参数")
			}
			result, err := fetchCategoryRooms(ctx, sc, areaID, page)
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{Items: result.Items, HasMore: platform.Bool(result.HasMore)}, nil
		},
	})
}

// newCategoriesChannel 分区目录(无翻页);fetchedAt 在请求前取值(TS 参数求值顺序)。
func newCategoriesChannel(sc *platform.ServiceContext) platform.DataChannel {
	return &simpleChannel{
		key:         "live:douyu:categories",
		name:        "斗鱼分区列表",
		kind:        platform.KindCategory,
		defaultInfo: platform.SourceInfo{},
		fetch: func(ctx context.Context, info platform.SourceInfo) (platform.FetchResult, error) {
			t := sc.NowMs()
			tree, err := fetchCategories(ctx, sc)
			if err != nil {
				return platform.FetchResult{}, err
			}
			return platform.FetchResult{Items: categoryItems("live:douyu:categories", t, tree)}, nil
		},
	}
}

// newAnchorChannel 搜主播(页码翻页)。
func newAnchorChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "live:douyu:anchor",
		Name: "斗鱼搜主播",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{{
			Key: "keyword", Label: "主播关键词", Required: platform.Bool(true),
		}},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(jsString(info["keyword"]))
			if keyword == "" {
				return platform.PageItems{}, errors.New("douyu:anchor 需要 keyword 参数")
			}
			items, err := searchAnchors(ctx, sc, keyword, page, anchorPageSize)
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}
