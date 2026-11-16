// 上游结构 → feed 契约的纯映射。取值语义严格对齐 TS:
// 空串/缺字段的取舍在每个函数里显式表达(在线数恒输出、主播桶不输出 online、
// 房间 channel 的 thumbnail/author 恒存在等)。

package huya

import (
	"strings"

	"mygo-host/platform"
)

func strVal(v *flexStr) string {
	if v == nil {
		return ""
	}
	return string(*v)
}

func numVal(v *flexNum) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

// ossCover 封面无 query 时补 OSS 压缩样式(省带宽)。
func ossCover(raw string) string {
	if raw != "" && !strings.Contains(raw, "?") {
		return raw + "?x-oss-process=style/w338_h190"
	}
	return raw
}

func liveStatusPtr(s platform.LiveStatus) *platform.LiveStatus { return &s }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// mapLiveRow 直播列表行 → Live item(热门 / 分区共用)。
func mapLiveRow(row liveRow, sourceID string, t int64) platform.Item {
	profileRoom := strVal(row.ProfileRoom)
	item := platform.Item{
		ID:         "huya:" + profileRoom,
		SourceID:   sourceID,
		Kind:       platform.KindLive,
		Title:      firstNonEmpty(strVal(row.Introduction), strVal(row.RoomName)),
		FetchedAt:  t,
		URL:        platform.Str("https://www.huya.com/" + profileRoom),
		Thumbnail:  platform.StrOmit(ossCover(strVal(row.Screenshot))),
		Platform:   "huya",
		RoomID:     platform.Str(profileRoom),
		LiveStatus: liveStatusPtr(platform.LiveStatusLive),
		Online:     platform.Int(numVal(row.TotalCount)),
	}
	if nick := strVal(row.Nick); nick != "" {
		item.Author = &platform.Author{Name: nick}
	}
	return item
}

// mapRoom profileRoom → Live item(URL 用订阅 roomID,而非上游 profileRoom)。
func mapRoom(roomID string, data *profileRoomData, t int64) platform.Item {
	ld := data.LiveData
	profileRoom, title, screenshot, nick := roomID, "", "", ""
	totalCount := 0
	var introduction *flexStr
	if ld != nil {
		if ld.ProfileRoom != nil {
			profileRoom = strVal(ld.ProfileRoom)
		}
		if ld.Introduction != nil {
			title = strVal(ld.Introduction)
		} else {
			title = strVal(ld.RoomName)
		}
		screenshot, nick = strVal(ld.Screenshot), strVal(ld.Nick)
		totalCount, introduction = numVal(ld.TotalCount), ld.Introduction
	}
	return platform.Item{
		ID:           "huya:" + profileRoom,
		SourceID:     "live:huya",
		Kind:         platform.KindLive,
		Title:        title,
		FetchedAt:    t,
		URL:          platform.Str("https://www.huya.com/" + roomID),
		Thumbnail:    platform.Str(screenshot),
		Author:       &platform.Author{Name: nick},
		Platform:     "huya",
		RoomID:       platform.Str(profileRoom),
		LiveStatus:   liveStatusPtr(roomLiveStatus(strVal(data.LiveStatus))),
		Online:       platform.Int(totalCount),
		Introduction: platform.StrOmit(strVal(introduction)),
	}
}

func roomLiveStatus(raw string) platform.LiveStatus {
	if strings.ToUpper(raw) == "ON" {
		return platform.LiveStatusLive
	}
	return platform.LiveStatusOffline
}

// mapAnchorRow 主播搜索结果行 → Live item(主播桶没有观众数,不输出 online)。
func mapAnchorRow(row searchRow, t int64) *platform.Item {
	roomID := strVal(row.RoomID)
	if roomID == "" {
		return nil
	}
	nick := strVal(row.GameNick)
	status := platform.LiveStatusOffline
	if row.GameLiveOn != nil && bool(*row.GameLiveOn) {
		status = platform.LiveStatusLive
	}
	item := platform.Item{
		ID:         "huya:" + roomID,
		SourceID:   "live:huya:anchor",
		Kind:       platform.KindLive,
		Title:      nick,
		FetchedAt:  t,
		URL:        platform.Str("https://www.huya.com/" + roomID),
		Thumbnail:  platform.StrOmit(strVal(row.GameAvatarURL)),
		Platform:   "huya",
		RoomID:     platform.Str(roomID),
		LiveStatus: liveStatusPtr(status),
	}
	if nick != "" {
		item.Author = &platform.Author{Name: nick}
	}
	return &item
}

// mapSearchRoom 房间搜索结果行 → Live item(room_id 可能错,用主播桶修正)。
func mapSearchRoom(row searchRow, anchors []searchRow, t int64) *platform.Item {
	roomID := findRoomID(row, anchors)
	if roomID == "" {
		return nil
	}
	nick := strVal(row.GameNick)
	item := platform.Item{
		ID:         "huya:" + roomID,
		SourceID:   "live:huya:search",
		Kind:       platform.KindLive,
		Title:      firstNonEmpty(strVal(row.GameIntroduction), strVal(row.GameRoomName)),
		FetchedAt:  t,
		URL:        platform.Str("https://www.huya.com/" + roomID),
		Thumbnail:  platform.StrOmit(ossCover(strVal(row.GameScreenshot))),
		Platform:   "huya",
		RoomID:     platform.Str(roomID),
		LiveStatus: liveStatusPtr(platform.LiveStatusLive),
		Online:     platform.Int(numVal(row.GameTotalCount)),
	}
	if nick != "" {
		item.Author = &platform.Author{Name: nick}
	}
	return &item
}

// findRoomID 用 uid+yyid 在主播桶里找真实 room_id,找不到回退原值。
func findRoomID(row searchRow, anchors []searchRow) string {
	fallback := strVal(row.RoomID)
	uid, yyid := strVal(row.UID), strVal(row.YYID)
	for _, anchor := range anchors {
		if strVal(anchor.UID) != uid || strVal(anchor.YYID) != yyid {
			continue
		}
		if id := strVal(anchor.RoomID); id != "" {
			return id
		}
		break
	}
	return fallback
}

// mapCategoryRows 二级分区 → Category item(点开用 live:huya:category + gameId)。
func mapCategoryRows(sourceID string, t int64, subs []subCategory) []platform.Item {
	items := make([]platform.Item, 0, len(subs))
	for _, sub := range subs {
		items = append(items, platform.Item{
			ID:              "huya-game:" + sub.ID,
			SourceID:        sourceID,
			Kind:            platform.KindCategory,
			Title:           sub.Name,
			Summary:         platform.Str(sub.ParentName),
			FetchedAt:       t,
			CategoryChannel: platform.Str("live:huya:category"),
			CategoryInfo:    map[string]string{"gameId": sub.ID},
		})
	}
	return items
}
