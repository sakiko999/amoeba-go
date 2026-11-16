// 虎牙发现层:顶层/二级分区与搜索(房间、主播)。
// 顶层 4 个分区是平台固定枚举(无接口);二级分区走 bussLive;搜索端点零鉴权,
// 房间桶的 room_id 可能错,用主播桶(uid+yyid)修正。

package huya

import (
	"context"
	"fmt"
	"net/url"

	"mygo-host/platform"
)

// topCategory 顶层分区(id 兼作 bussType 与 gameId)。
type topCategory struct{ ID, Name string }

var topCategories = []topCategory{
	{ID: "1", Name: "网游"},
	{ID: "2", Name: "单机"},
	{ID: "8", Name: "娱乐"},
	{ID: "3", Name: "手游"},
}

// subCategory 二级分区(id = gid,作 gameId 列房间)。
type subCategory struct {
	ID         string
	Name       string
	ParentID   string
	ParentName string
}

// fetchSubCategories 列某顶层分区下的二级分区;单个分区失败由调用方隔离。
func fetchSubCategories(ctx context.Context, sc *platform.ServiceContext, parentID string) ([]subCategory, error) {
	endpoint := bussLiveAPI + "?bussType=" + url.QueryEscape(parentID)
	raw, err := sc.HTTP.GetJSON(ctx, endpoint, map[string]string{
		"user-agent": uaAndroid,
		"referer":    "https://www.huya.com/",
	})
	if err != nil {
		return nil, err
	}
	var res bussLiveResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	return mapSubCategoryRows(parentID, res.Data), nil
}

// mapSubCategoryRows 过滤空 gid/name,并挂上顶层分区名(TS 侧同类过滤)。
func mapSubCategoryRows(parentID string, rows []subCategoryRow) []subCategory {
	parentName := ""
	for _, top := range topCategories {
		if top.ID == parentID {
			parentName = top.Name
			break
		}
	}
	subs := make([]subCategory, 0, len(rows))
	for _, row := range rows {
		id, name := strVal(row.Gid), strVal(row.GameFullName)
		if id == "" || name == "" {
			continue
		}
		subs = append(subs, subCategory{ID: id, Name: name, ParentID: parentID, ParentName: parentName})
	}
	return subs
}

// fetchSearch 调搜索端点;version: v=4 房间桶 / v=1 主播桶。
func fetchSearch(ctx context.Context, sc *platform.ServiceContext, keyword string, version, rows, start int) (*searchResponse, error) {
	endpoint := fmt.Sprintf("%s&q=%s&uid=0&v=%d&typ=-5&livestate=0&rows=%d&start=%d",
		searchAPI, url.QueryEscape(keyword), version, rows, start)
	raw, err := sc.HTTP.GetJSON(ctx, endpoint, map[string]string{
		"user-agent": uaDesktop,
		"referer":    "https://www.huya.com/",
	})
	if err != nil {
		return nil, err
	}
	var res searchResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *searchResponse) bucket(key string) []searchRow {
	if r == nil || r.Response == nil || r.Response[key] == nil {
		return nil
	}
	return r.Response[key].Docs
}

// searchAnchors 搜主播(v=1;rows 服务端封顶 50,start 语义怪异故不接翻页)。
func searchAnchors(ctx context.Context, sc *platform.ServiceContext, keyword string, pageSize int) ([]platform.Item, error) {
	res, err := fetchSearch(ctx, sc, keyword, 1, clampInt(pageSize, 1, 50), 0)
	if err != nil {
		return nil, err
	}
	rows := res.bucket("1")
	t := sc.NowMs()
	items := make([]platform.Item, 0, len(rows))
	for _, row := range rows {
		if item := mapAnchorRow(row, t); item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

// searchRooms 搜房间(v=4;rows 上限 100,start 为 (page-1)*rows)。
func searchRooms(ctx context.Context, sc *platform.ServiceContext, keyword string, page, pageSize int) ([]platform.Item, error) {
	rows := clampInt(pageSize, 1, 100)
	res, err := fetchSearch(ctx, sc, keyword, 4, rows, (page-1)*rows)
	if err != nil {
		return nil, err
	}
	rooms, anchors := res.bucket("3"), res.bucket("1")
	t := sc.NowMs()
	items := make([]platform.Item, 0, len(rooms))
	for _, row := range rooms {
		if item := mapSearchRoom(row, anchors, t); item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
