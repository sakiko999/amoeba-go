// 映射层测试:上游 JSON → Item 的字段与 JSON 形状对齐 TS(黄金集内联 fixture)。

package douyu

import (
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

const testNow = int64(1700000000000)

func TestMapRoomItem(t *testing.T) {
	room := &roomPayload{
		RoomName:    "房间A",
		RoomPic:     "https://pic/a.jpg",
		OwnerName:   "主播A",
		OwnerAvatar: "",
		RoomID:      float64(9527),
		ShowStatus:  float64(1),
		VideoLoop:   float64(0),
		RoomBizAll:  &roomBiz{Hot: "133.1万"},
		ShowDetails: "简介",
	}
	item := mapRoomItem("123", room, testNow)
	if item.ID != "douyu:123" || item.SourceID != "live:douyu" || item.Kind != platform.KindLive {
		t.Errorf("identity = %+v", item)
	}
	if item.Title != "房间A" || item.URL == nil || *item.URL != "https://www.douyu.com/123" ||
		item.FetchedAt != testNow {
		t.Errorf("title/url/fetchedAt = %+v", item)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://pic/a.jpg" ||
		item.Author == nil || item.Author.Name != "主播A" || item.Author.Avatar != nil {
		t.Errorf("thumbnail/author = %v / %+v", item.Thumbnail, item.Author)
	}
	if item.RoomID == nil || *item.RoomID != "9527" ||
		item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("roomId/liveStatus = %v / %v", item.RoomID, item.LiveStatus)
	}
	if item.IsRecord == nil || *item.IsRecord || item.Online != nil {
		t.Errorf("isRecord/online = %v / %v(单位串 hot 不应产出 online)", item.IsRecord, item.Online)
	}
	if item.Introduction == nil || *item.Introduction != "简介" {
		t.Errorf("introduction = %v", item.Introduction)
	}
}

func TestMapRoomItemOfflineRecording(t *testing.T) {
	room := &roomPayload{
		RoomName:   "录播房",
		RoomPic:    "",
		OwnerName:  "主播B",
		ShowStatus: "0",
		VideoLoop:  "1",
		RoomBizAll: &roomBiz{Hot: float64(12345)},
	}
	item := mapRoomItem("456", room, testNow)
	if *item.LiveStatus != platform.LiveStatusOffline {
		t.Errorf("liveStatus = %v", *item.LiveStatus)
	}
	if !*item.IsRecord {
		t.Error("isRecord want true")
	}
	if item.Online == nil || *item.Online != 12345 {
		t.Errorf("online = %v", item.Online)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "" {
		t.Errorf("空 room_pic 应保留空串(TS String): %v", item.Thumbnail)
	}
	if item.RoomID == nil || *item.RoomID != "456" {
		t.Errorf("roomId fallback = %v", item.RoomID)
	}
}

func TestMapHotItems(t *testing.T) {
	list := []map[string]any{
		{"rid": float64(1), "rn": "A", "type": float64(1), "rs16": "https://pic/1", "nn": "主播A", "ol": "133.1万"},
		{"rid": float64(2), "rn": "广告", "type": float64(2)},
		{"rid": float64(3), "rn": "B", "ol": float64(7)},
	}
	items := mapHotItems(list, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ID != "douyu:1" || items[0].Online == nil || *items[0].Online != 1331000 {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[0].Thumbnail == nil || items[0].Author == nil || items[0].Author.Name != "主播A" {
		t.Errorf("items[0] media = %+v", items[0])
	}
	if items[1].ID != "douyu:3" || items[1].Online == nil || *items[1].Online != 7 {
		t.Errorf("items[1] = %+v", items[1])
	}
}

func TestMapCategoryRoomItems(t *testing.T) {
	list := []map[string]any{
		{"rid": float64(11), "rn": "A", "type": "1", "ol": "1234"},
		{"rid": float64(12), "rn": "广告", "type": "0"},
		{"rid": float64(13), "rn": "B", "type": float64(1), "ol": "1.3万"},
	}
	items := mapCategoryRoomItems(list, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].SourceID != "live:douyu:category" || items[0].Online == nil || *items[0].Online != 1234 {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[1].Online != nil {
		t.Errorf("非数值 ol 不应产出 online: %v", *items[1].Online)
	}
}

func TestMapAnchorItems(t *testing.T) {
	list := []map[string]any{
		{"anchorInfo": map[string]any{"rid": float64(10), "nickName": "主播", "isLive": float64(1), "roomType": float64(0), "avatar": "https://a"}},
		{"anchorInfo": map[string]any{"rid": float64(11), "nickName": "录播", "isLive": float64(1), "roomType": float64(1)}},
		{"anchorInfo": map[string]any{}},
	}
	items := mapAnchorItems(list, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if *items[0].LiveStatus != platform.LiveStatusLive || items[0].Online != nil {
		t.Errorf("items[0] = %+v", items[0])
	}
	if *items[1].LiveStatus != platform.LiveStatusOffline {
		t.Errorf("items[1] liveStatus = %v", *items[1].LiveStatus)
	}
}

func TestMapSearchRoomItems(t *testing.T) {
	list := []map[string]any{
		{"rid": float64(21), "roomName": "房", "isLive": float64(1), "roomType": float64(0), "hot": "2.5万"},
		{"rid": float64(22), "roomName": "录播", "isLive": float64(0), "roomType": float64(0)},
		{"roomName": "无 id"},
	}
	items := mapSearchRoomItems(list, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].Online == nil || *items[0].Online != 25000 {
		t.Errorf("online = %v", items[0].Online)
	}
	if *items[1].LiveStatus != platform.LiveStatusOffline || items[1].Online != nil {
		t.Errorf("items[1] = %+v", items[1])
	}
}

func TestCategoryItems(t *testing.T) {
	tree := []douyuCategory{
		{ID: "1", Name: "网游", Children: []douyuChild{{ID: "1", Name: "英雄联盟"}, {ID: "2", Name: "DOTA2"}}},
		{ID: "2", Name: "手游", Children: []douyuChild{{ID: "3", Name: "王者荣耀"}}},
	}
	items := categoryItems("live:douyu:categories", testNow, tree)
	if len(items) != 3 {
		t.Fatalf("items = %d", len(items))
	}
	first := items[0]
	if first.ID != "douyu-area:1" || first.SourceID != "live:douyu:categories" || first.Kind != platform.KindCategory {
		t.Errorf("items[0] identity = %+v", first)
	}
	if first.Title != "英雄联盟" || first.Summary == nil || *first.Summary != "网游" {
		t.Errorf("items[0] title/summary = %+v", first)
	}
	if first.CategoryChannel == nil || *first.CategoryChannel != "live:douyu:category" ||
		first.CategoryInfo["areaId"] != "1" {
		t.Errorf("items[0] category = %+v", first)
	}
}

func TestItemJSONShape(t *testing.T) {
	item := mapHotItems([]map[string]any{{"rid": float64(1), "rn": "A", "type": float64(1)}}, testNow)[0]
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, key := range []string{`"sourceId"`, `"kind"`, `"title"`, `"url"`, `"fetchedAt"`, `"platform"`, `"roomId"`, `"liveStatus"`} {
		if !strings.Contains(text, key) {
			t.Errorf("json 缺少 %s: %s", key, text)
		}
	}
	if strings.Contains(text, `"summary"`) || strings.Contains(text, `"publishedAt"`) || strings.Contains(text, `"online"`) {
		t.Errorf("json 多出可选字段: %s", text)
	}
}

func TestParseRoomIds(t *testing.T) {
	ids := parseRoomIds(platform.SourceInfo{"roomIds": "1,2，3 4\n5"})
	want := []string{"1", "2", "3", "4", "5"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
	if got := parseRoomIds(platform.SourceInfo{}); len(got) != 0 {
		t.Errorf("空 roomIds = %v", got)
	}
}
