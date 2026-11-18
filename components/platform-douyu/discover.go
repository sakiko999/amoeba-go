// douyu 发现层:分区树、按分区列房、房间/主播搜索(均匿名可用)。
// 对齐 TS lib/discover.ts;page 从 1 起,分页页码在 path/query 里。

package douyu

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

const discoverUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

// douyuCategory 一级分区(含二级)。
type douyuCategory struct {
	ID       string
	Name     string
	Children []douyuChild
}

type douyuChild struct {
	ID   string
	Name string
}

// categoryRoomsResult 按分区列房的结果(过滤后条目 + 原始页推断的 hasMore)。
type categoryRoomsResult struct {
	Items   []platform.Item
	HasMore bool
}

func searchHeaders() map[string]string {
	return map[string]string{
		"user-agent": discoverUA,
		"origin":     "https://www.douyu.com",
		"referer":    "https://www.douyu.com/search/",
	}
}

// fetchCategories 一次拉全部一级/二级分区,内存按 cate1Id 关联。
func fetchCategories(ctx context.Context, sc *platform.ServiceContext) ([]douyuCategory, error) {
	raw, err := sc.HTTP.GetJSON(ctx, mobileBaseURL+"/api/cate/list",
		map[string]string{"user-agent": discoverUA, "referer": baseURL + "/"})
	if err != nil {
		return nil, err
	}
	var res categoryListResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	tree := make([]douyuCategory, 0, len(res.Data.Cate1Info))
	for _, c1 := range res.Data.Cate1Info {
		id := platform.JSString(c1["cate1Id"])
		category := douyuCategory{ID: id, Name: platform.JSString(c1["cate1Name"])}
		for _, c2 := range res.Data.Cate2Info {
			if platform.JSString(c2["cate1Id"]) != id {
				continue
			}
			category.Children = append(category.Children, douyuChild{
				ID:   platform.JSString(c2["cate2Id"]),
				Name: platform.JSString(c2["cate2Name"]),
			})
		}
		if id == "" {
			continue
		}
		tree = append(tree, category)
	}
	return tree, nil
}

// fetchCategoryRooms 按分区列房间;pgcnt 可用时精确判页,否则按原始行非空推断。
func fetchCategoryRooms(ctx context.Context, sc *platform.ServiceContext, areaID string, page int) (categoryRoomsResult, error) {
	url := fmt.Sprintf("%s/gapi/rkc/directory/mixList/2_%s/%d", baseURL, encodeURIComponent(areaID), page)
	raw, err := sc.HTTP.GetJSON(ctx, url, map[string]string{"user-agent": discoverUA, "referer": baseURL + "/"})
	if err != nil {
		return categoryRoomsResult{}, err
	}
	var res directoryResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return categoryRoomsResult{}, err
	}
	list := res.Data.Rl
	items := mapCategoryRoomItems(list, sc.NowMs())
	pgcnt, pgOK := platform.JSNumber(res.Data.Pgcnt)
	hasMore := len(list) > 0
	if pgOK && pgcnt > 0 {
		hasMore = float64(page) < pgcnt
	}
	return categoryRoomsResult{Items: items, HasMore: hasMore}, nil
}

func clampPageSize(pageSize int) int {
	if pageSize < 1 {
		return 1
	}
	if pageSize > 50 {
		return 50
	}
	return pageSize
}

// searchAnchors 搜索主播(匿名可用)。
func searchAnchors(ctx context.Context, sc *platform.ServiceContext, keyword string, page, pageSize int) ([]platform.Item, error) {
	size := clampPageSize(pageSize)
	url := fmt.Sprintf("%s/japi/search/api/searchUser?kw=%s&page=%d&pageSize=%d&filterType=1",
		baseURL, encodeURIComponent(keyword), page, size)
	raw, err := sc.HTTP.GetJSON(ctx, url, searchHeaders())
	if err != nil {
		return nil, err
	}
	var res searchUserResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	return mapAnchorItems(res.Data.RelateUser, sc.NowMs()), nil
}

// searchRooms 搜索房间(当前无渠道消费,保留平台导出能力)。
func searchRooms(ctx context.Context, sc *platform.ServiceContext, keyword string, page, pageSize int) ([]platform.Item, error) {
	size := clampPageSize(pageSize)
	url := fmt.Sprintf("%s/japi/search/api/searchShow?kw=%s&page=%d&pageSize=%d",
		baseURL, encodeURIComponent(keyword), page, size)
	raw, err := sc.HTTP.GetJSON(ctx, url, searchHeaders())
	if err != nil {
		return nil, err
	}
	var res searchShowResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	return mapSearchRoomItems(res.Data.RelateShow, sc.NowMs()), nil
}
