// bili 直播渠道：房间详情（多房间聚合）/ 热门 / 分区目录 / 按分区列房。
// 分区列房必须登录（未登录必得 -352，提前给出登录指引）；其余零登录可用。

package bili

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"mygo-host/platform"
)

var roomIDsSplitRe = regexp.MustCompile(`[,，\s]+`)

// parseRoomIDs roomIds 逗号/空白分隔 → 去空列表。
func parseRoomIDs(info platform.SourceInfo) []string {
	var out []string
	for _, part := range roomIDsSplitRe.Split(info["roomIds"], -1) {
		if id := strings.TrimSpace(part); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func liveChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:live",
		Name: "B站直播房间",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{
			requiredField("roomIds", "直播间 ID(逗号分隔,可多个)"),
			optionalField("cookie", "登录 cookie(可选)"),
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			roomIDs := parseRoomIDs(info)
			if len(roomIDs) == 0 {
				return platform.PageItems{}, fmt.Errorf("bili:live 需要 roomIds")
			}
			t := c.now()
			results := make([]*platform.Item, len(roomIDs))
			var wg sync.WaitGroup
			for i, roomID := range roomIDs {
				wg.Add(1)
				go func(i int, roomID string) {
					defer wg.Done()
					item, err := c.fetchBiliRoom(ctx, roomID, cookiePtr(info))
					if err != nil {
						c.log("bili: 房间 %s 拉取失败: %v", roomID, err)
						return
					}
					results[i] = item
				}(i, roomID)
			}
			wg.Wait()
			items := make([]platform.Item, 0, len(roomIDs))
			for _, item := range results {
				if item != nil {
					item.FetchedAt = t
					items = append(items, *item)
				}
			}
			// 全 null 只可能是请求/风控失败（离线房间仍返回 item），必须抛错。
			if len(items) == 0 {
				return platform.PageItems{}, fmt.Errorf("bili:live 全部房间拉取失败(%d 个)", len(roomIDs))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

// fetchBiliRoom 单房间 → Live item（不抛错：一个失败不拖垮多房间订阅）。
func (c *client) fetchBiliRoom(ctx context.Context, roomID string, cookie *string) (*platform.Item, error) {
	params, err := c.signLiveParams(ctx, map[string]string{"room_id": roomID}, cookie)
	if err != nil {
		return nil, err
	}
	url := "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom?" + params
	_, data, err := c.getJSON(ctx, url, reqOptions{Referer: liveBase + "/", Buvid: true, Cookie: cookie})
	if err != nil {
		return nil, err
	}
	payload := jobj(data["data"])
	ri := jobj(payload["room_info"])
	ai := jobj(payload["anchor_info"])
	baseInfo := jobj(ai["base_info"])

	realRoomID := roomID
	if v := ri["room_id"]; v != nil {
		realRoomID = jstr(v)
	}
	item := platform.Item{
		ID:         "bilibili:" + realRoomID,
		SourceID:   "bili:live",
		Kind:       platform.KindLive,
		Title:      jstr(ri["title"]),
		URL:        platform.Str(liveBase + "/" + realRoomID),
		Thumbnail:  platform.Str(jstr(ri["cover"])),
		Author:     &platform.Author{Name: jstr(baseInfo["uname"])},
		Platform:   "bilibili",
		RoomID:     platform.Str(realRoomID),
		LiveStatus: liveStatusPtr(jnum(ri["live_status"]) == 1),
		Online:     platform.Int(jint(ri["online"])),
	}
	if jtruthy(ri["description"]) {
		item.Introduction = platform.Str(jstr(ri["description"]))
	}
	if jtruthy(ri["live_start_time"]) {
		sec := jnum(ri["live_start_time"])
		if !finite(sec) {
			return nil, fmt.Errorf("bili:live 房间 %s 的 live_start_time 非法", roomID)
		}
		item.ShowTime = platform.Str(time.UnixMilli(int64(sec * 1000)).UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	return &item, nil
}

func liveHotChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:      "bili:live:hot",
		Name:     "B站直播热门",
		Kind:     platform.KindLive,
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			query := fmt.Sprintf("platform=web&sort=online&page_size=30&page=%d", page)
			signed, err := c.signWeb(ctx, query, cookiePtr(info))
			if err != nil {
				return platform.PageItems{}, err
			}
			url := "https://api.live.bilibili.com/xlive/web-interface/v1/second/getListByArea?" + signed
			// 无 buvid 的请求必 -352（wbi 签名齐全也没用）。
			_, data, err := c.getJSON(ctx, url, reqOptions{
				Referer: liveBase + "/", Buvid: true, Cookie: cookiePtr(info),
			})
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{Items: liveRows(data, "bili:live:hot", c.now())}, nil
		},
	})
}

func liveAreasChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:         "bili:live:areas",
		Name:        "B站直播分区列表",
		Kind:        platform.KindCategory,
		DefaultInfo: platform.SourceInfo{},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			// TS 的实参求值顺序：now() 先于网络请求。
			t := c.now()
			tree, err := c.fetchBiliCategories(ctx)
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{Items: biliCategoryItems("bili:live:areas", t, tree)}, nil
		},
	})
}

// biliCategoryItems 分区树 → 目录条目（跳过空 id 子分区）。
func biliCategoryItems(sourceID string, t int64, tree []biliCategory) []platform.Item {
	items := make([]platform.Item, 0)
	push := func(id, title, summary, parentID, areaID string) {
		items = append(items, platform.Item{
			ID:              id,
			SourceID:        sourceID,
			Kind:            platform.KindCategory,
			Title:           title,
			URL:             platform.Str(fmt.Sprintf("%s/p/eden/area-tags?parentAreaId=%s&areaId=%s", liveBase, parentID, areaID)),
			Summary:         platform.Str(summary),
			FetchedAt:       t,
			CategoryChannel: platform.Str("bili:live:area"),
			CategoryInfo:    map[string]string{"parentAreaId": parentID, "areaId": areaID},
		})
	}
	for _, parent := range tree {
		summary := "顶层分区"
		if len(parent.Children) > 0 {
			summary = fmt.Sprintf("%d 个子分区", len(parent.Children))
		}
		push("bili-area:"+parent.ID, parent.Name, summary, parent.ID, "0")
		for _, sub := range parent.Children {
			if sub.ID == "" {
				continue
			}
			push("bili-area:"+parent.ID+":"+sub.ID, sub.Name, parent.Name+" · 子分区", parent.ID, sub.ID)
		}
	}
	return items
}

func liveAreaChannel(c *client) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "bili:live:area",
		Name: "B站分区直播",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{
			requiredField("parentAreaId", "父分区 ID(如 2=网游、6=手游)"),
			{Key: "areaId", Label: "子分区 ID(0=该分区全部)", Placeholder: platform.Str("0")},
		},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			parentAreaID := strings.TrimSpace(info["parentAreaId"])
			areaID := strings.TrimSpace(info["areaId"])
			if areaID == "" {
				areaID = "0"
			}
			if parentAreaID == "" {
				return platform.PageItems{}, fmt.Errorf("bili:live:area 需要 parentAreaId 参数")
			}
			rooms, err := c.fetchBiliAreaRooms(ctx, parentAreaID, areaID, page, cookiePtr(info))
			if err != nil {
				// 上游 API 的 -352 = 未登录/凭据失效 → 改写为登录指引。
				if strings.Contains(err.Error(), "bilibili API -352") {
					return platform.PageItems{}, fmt.Errorf("B站直播分区列房需要登录：请到「账号与凭证」扫码登录")
				}
				return platform.PageItems{}, err
			}
			return platform.PageItems{
				Items:   liveRowsFromList(rooms.List, "bili:live:area", c.now()),
				HasMore: platform.Bool(rooms.HasMore),
			}, nil
		},
	})
}

// liveRows getListByArea 响应 → Live item（data.list）。
func liveRows(data map[string]any, sourceID string, t int64) []platform.Item {
	return liveRowsFromList(jlist(jobj(data["data"])["list"]), sourceID, t)
}

// liveRowsFromList 直播房间行 → Live item（hot / area 同形：roomId/title/uname/cover/online）。
func liveRowsFromList(list []any, sourceID string, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		row := jobj(raw)
		if row == nil {
			continue
		}
		roomID := jstr(row["roomid"])
		cover := ""
		if jtruthy(row["cover"]) {
			cover = jstr(row["cover"])
		}
		item := platform.Item{
			ID:         "bilibili:" + roomID,
			SourceID:   sourceID,
			Kind:       platform.KindLive,
			Title:      jstr(row["title"]),
			URL:        platform.Str(liveBase + "/" + roomID),
			FetchedAt:  t,
			Platform:   "bilibili",
			RoomID:     platform.Str(roomID),
			LiveStatus: liveStatusPtr(true),
			Online:     platform.Int(jint(row["online"])),
		}
		if cover != "" {
			item.Thumbnail = platform.Str(cover)
		}
		if jtruthy(row["uname"]) {
			item.Author = &platform.Author{Name: jstr(row["uname"])}
		}
		items = append(items, item)
	}
	return items
}
