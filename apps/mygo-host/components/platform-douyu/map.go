// 上游记录 → platform.Item 的纯映射(不碰网络;fixture 单测覆盖)。
// 字段访问全部走 helpers 的 JS 语义,与 TS 的 ?? / truthy / String 对齐。

package douyu

import (
	"math"

	"mygo-host/platform"
)

// mapRoomItem 单房间记录 → Live item(id 用订阅参数,roomId 优先上游值)。
func mapRoomItem(roomID string, room *roomPayload, t int64) platform.Item {
	roomIDValue := roomID
	if room.RoomID != nil {
		roomIDValue = jsString(room.RoomID)
	}
	showStatus, showOK := toInt(room.ShowStatus)
	videoLoop, loopOK := toInt(room.VideoLoop)
	status := platform.LiveStatusOffline
	if showOK && showStatus == 1 && !(loopOK && videoLoop == 1) {
		status = platform.LiveStatusLive
	}
	var online *int
	if room.RoomBizAll != nil {
		if n, ok := toInt(room.RoomBizAll.Hot); ok {
			online = platform.Int(int(n))
		}
	}
	return platform.Item{
		ID:           "douyu:" + roomID,
		SourceID:     "live:douyu",
		Kind:         platform.KindLive,
		Title:        jsString(room.RoomName),
		URL:          platform.Str(baseURL + "/" + roomID),
		Thumbnail:    platform.Str(jsString(room.RoomPic)),
		Author:       &platform.Author{Name: jsString(room.OwnerName), Avatar: strOr(room.OwnerAvatar)},
		FetchedAt:    t,
		Platform:     "douyu",
		RoomID:       platform.Str(roomIDValue),
		LiveStatus:   liveStatusPtr(status),
		Online:       online,
		IsRecord:     platform.Bool(loopOK && videoLoop == 1),
		Introduction: strOr(room.ShowDetails),
	}
}

// mapHotItems 热门页 data.rl → Live items(过滤非直播间广告位)。
func mapHotItems(list []map[string]any, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		if !isRoomType(raw["type"]) {
			continue
		}
		rid := jsString(raw["rid"])
		items = append(items, platform.Item{
			ID:         "douyu:" + rid,
			SourceID:   "live:douyu:hot",
			Kind:       platform.KindLive,
			Title:      jsString(raw["rn"]),
			URL:        platform.Str(baseURL + "/" + rid),
			Thumbnail:  truthyString(raw["rs16"]),
			Author:     truthyAuthor(raw["nn"]),
			FetchedAt:  t,
			Platform:   "douyu",
			RoomID:     platform.Str(rid),
			LiveStatus: liveStatusPtr(platform.LiveStatusLive),
			Online:     parseCount(raw["ol"]),
		})
	}
	return items
}

// mapCategoryRoomItems 分区房间列表 → Live items(过滤非直播间广告位)。
func mapCategoryRoomItems(list []map[string]any, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		if !isRoomType(raw["type"]) {
			continue
		}
		rid := jsString(raw["rid"])
		items = append(items, platform.Item{
			ID:         "douyu:" + rid,
			SourceID:   "live:douyu:category",
			Kind:       platform.KindLive,
			Title:      jsString(raw["rn"]),
			URL:        platform.Str(baseURL + "/" + rid),
			Thumbnail:  truthyString(raw["rs16"]),
			Author:     truthyAuthor(raw["nn"]),
			FetchedAt:  t,
			Platform:   "douyu",
			RoomID:     platform.Str(rid),
			LiveStatus: liveStatusPtr(platform.LiveStatusLive),
			Online:     numberOrNil(raw["ol"]),
		})
	}
	return items
}

// mapAnchorItems 搜主播 data.relateUser → Live items(anchorInfo 无 hot 字段,不报 online)。
func mapAnchorItems(list []map[string]any, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		anchor, _ := raw["anchorInfo"].(map[string]any)
		if anchor == nil {
			anchor = map[string]any{}
		}
		rid := jsString(anchor["rid"])
		if rid == "" {
			continue
		}
		items = append(items, platform.Item{
			ID:         "douyu:" + rid,
			SourceID:   "live:douyu:anchor",
			Kind:       platform.KindLive,
			Title:      jsString(anchor["nickName"]),
			URL:        platform.Str(baseURL + "/" + rid),
			Thumbnail:  truthyString(anchor["avatar"]),
			Author:     truthyAuthor(anchor["nickName"]),
			FetchedAt:  t,
			Platform:   "douyu",
			RoomID:     platform.Str(rid),
			LiveStatus: liveStatusPtr(liveStatusOf(anchor["isLive"], anchor["roomType"])),
		})
	}
	return items
}

// mapSearchRoomItems 搜房间 data.relateShow → Live items。
func mapSearchRoomItems(list []map[string]any, t int64) []platform.Item {
	items := make([]platform.Item, 0, len(list))
	for _, raw := range list {
		rid := jsString(raw["rid"])
		if rid == "" {
			continue
		}
		items = append(items, platform.Item{
			ID:         "douyu:" + rid,
			SourceID:   "live:douyu:search",
			Kind:       platform.KindLive,
			Title:      jsString(raw["roomName"]),
			URL:        platform.Str(baseURL + "/" + rid),
			Thumbnail:  truthyString(raw["roomSrc"]),
			Author:     truthyAuthor(raw["nickName"]),
			FetchedAt:  t,
			Platform:   "douyu",
			RoomID:     platform.Str(rid),
			LiveStatus: liveStatusPtr(liveStatusOf(raw["isLive"], raw["roomType"])),
			Online:     parseCount(raw["hot"]),
		})
	}
	return items
}

// categoryItems 分区树 → 目录条目(只列二级分区)。
func categoryItems(sourceID string, t int64, tree []douyuCategory) []platform.Item {
	items := make([]platform.Item, 0)
	for _, category := range tree {
		for _, sub := range category.Children {
			items = append(items, platform.Item{
				ID:              "douyu-area:" + sub.ID,
				SourceID:        sourceID,
				Kind:            platform.KindCategory,
				Title:           sub.Name,
				Summary:         platform.Str(category.Name),
				FetchedAt:       t,
				CategoryChannel: platform.Str("live:douyu:category"),
				CategoryInfo:    map[string]string{"areaId": sub.ID},
			})
		}
	}
	return items
}

// isRoomType 等价 Number(item.type ?? 1) === 1(缺失/空按 1 保留)。
func isRoomType(v any) bool {
	if v == nil {
		return true
	}
	n, ok := jsNumber(v)
	return ok && n == 1
}

// liveStatusOf 等价 isLive==1 && roomType==0。
func liveStatusOf(isLive, roomType any) platform.LiveStatus {
	if jsNumberOr(isLive, 0) == 1 && jsNumberOr(roomType, 0) == 0 {
		return platform.LiveStatusLive
	}
	return platform.LiveStatusOffline
}

// numberOrNil 等价 Number(v ?? 0)(NaN → 省略,消费者 nullish 语义一致)。
func numberOrNil(v any) *int {
	n := jsNumberOr(v, 0)
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return platform.Int(int(n))
}

func truthyString(v any) *string {
	if !truthy(v) {
		return nil
	}
	return platform.Str(jsString(v))
}

func truthyAuthor(v any) *platform.Author {
	if !truthy(v) {
		return nil
	}
	return &platform.Author{Name: jsString(v)}
}

func liveStatusPtr(status platform.LiveStatus) *platform.LiveStatus {
	return &status
}
