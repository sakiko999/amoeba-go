// 映射黄金集:testdata 为虎牙端点响应的真实结构(数字/字符串混给、缺字段),
// 断言与 TS 映射语义一致(标题回退、封面 OSS 后缀、online 恒输出等)。

package huya

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"mygo-host/platform"
)

func fixture(t *testing.T, name string) platform.RawJSON {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadLiveList(t *testing.T) []liveRow {
	t.Helper()
	var res liveListResponse
	if err := platform.UnmarshalJSON(fixture(t, "live_list.json"), &res); err != nil {
		t.Fatal(err)
	}
	if res.Data == nil || len(res.Data.Datas) != 2 {
		t.Fatalf("live_list datas = %+v", res.Data)
	}
	return res.Data.Datas
}

func TestMapLiveList(t *testing.T) {
	rows := loadLiveList(t)
	first := mapLiveRow(rows[0], "live:huya:hot", 42)
	if first.ID != "huya:11342412" || first.SourceID != "live:huya:hot" || first.Kind != platform.KindLive {
		t.Errorf("first = %+v", first)
	}
	if first.Title != "韩服王者局" || first.URL == nil || *first.URL != "https://www.huya.com/11342412" {
		t.Errorf("first title/url = %q / %v", first.Title, first.URL)
	}
	if first.Thumbnail == nil || *first.Thumbnail != "https://anchorpost.msstatic.com/a.jpg?x-oss-process=style/w338_h190" {
		t.Errorf("first thumbnail = %v", first.Thumbnail)
	}
	if first.Author == nil || first.Author.Name != "主播甲" {
		t.Errorf("first author = %+v", first.Author)
	}
	if first.Platform != "huya" || first.RoomID == nil || *first.RoomID != "11342412" {
		t.Errorf("first platform/room = %q / %v", first.Platform, first.RoomID)
	}
	if first.LiveStatus == nil || *first.LiveStatus != platform.LiveStatusLive || first.Online == nil || *first.Online != 12345 {
		t.Errorf("first status/online = %v / %v", first.LiveStatus, first.Online)
	}

	second := mapLiveRow(rows[1], "live:huya:hot", 42)
	if second.ID != "huya:660000" || second.Title != "房间二" {
		t.Errorf("second = %+v", second)
	}
	if second.Thumbnail == nil || *second.Thumbnail != "https://anchorpost.msstatic.com/b.jpg?x-oss-process=style/w338_h190" {
		t.Errorf("second thumbnail 不应重复补样式: %v", second.Thumbnail)
	}
	if second.Author != nil {
		t.Errorf("second author = %+v", second.Author)
	}
	if second.Online == nil || *second.Online != 7 {
		t.Errorf("second online = %v", second.Online)
	}
}

func loadProfile(t *testing.T, name string) *profileRoomData {
	t.Helper()
	var res profileRoomResponse
	if err := platform.UnmarshalJSON(fixture(t, name), &res); err != nil {
		t.Fatal(err)
	}
	if res.Data == nil {
		t.Fatal("profileRoom data 缺失")
	}
	return res.Data
}

func TestMapRoom(t *testing.T) {
	item := mapRoom("11342412", loadProfile(t, "profile_room.json"), 99)
	if item.ID != "huya:11342412" || item.Title != "直播简介" || item.FetchedAt != 99 {
		t.Errorf("item = %+v", item)
	}
	if item.URL == nil || *item.URL != "https://www.huya.com/11342412" {
		t.Errorf("url = %v", item.URL)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://anchorpost.msstatic.com/room.jpg" {
		t.Errorf("thumbnail = %v", item.Thumbnail)
	}
	if item.Author == nil || item.Author.Name != "主播甲" {
		t.Errorf("author = %+v", item.Author)
	}
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	if item.Online == nil || *item.Online != 8888 {
		t.Errorf("online = %v", item.Online)
	}
	if item.Introduction == nil || *item.Introduction != "直播简介" {
		t.Errorf("introduction = %v", item.Introduction)
	}
	raw, _ := json.Marshal(item)
	for _, key := range []string{`"thumbnail"`, `"introduction":"直播简介"`, `"online":8888`, `"liveStatus":"live"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("json 缺少 %s: %s", key, raw)
		}
	}
}

func TestMapRoomOffline(t *testing.T) {
	item := mapRoom("660000", loadProfile(t, "profile_room_offline.json"), 1)
	if item.Title != "离线房间" || item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusOffline {
		t.Errorf("item = %+v(status %v)", item, item.LiveStatus)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "" {
		t.Errorf("thumbnail 应存在且为空串: %v", item.Thumbnail)
	}
	if item.Author == nil || item.Author.Name != "" {
		t.Errorf("author 应存在且为空名: %+v", item.Author)
	}
	if item.Online == nil || *item.Online != 0 || item.Introduction != nil {
		t.Errorf("online/introduction = %v / %v", item.Online, item.Introduction)
	}
	raw, _ := json.Marshal(item)
	if !strings.Contains(string(raw), `"thumbnail":""`) || !strings.Contains(string(raw), `"online":0`) {
		t.Errorf("json 形状不符: %s", raw)
	}
	if strings.Contains(string(raw), `"introduction"`) {
		t.Errorf("introduction 不应输出: %s", raw)
	}
}

func TestMapCategoryRows(t *testing.T) {
	var res bussLiveResponse
	if err := platform.UnmarshalJSON(fixture(t, "buss_live.json"), &res); err != nil {
		t.Fatal(err)
	}
	subs := mapSubCategoryRows("1", res.Data)
	if len(subs) != 2 || subs[0].ID != "1" || subs[0].Name != "英雄联盟" || subs[0].ParentName != "网游" {
		t.Fatalf("subs = %+v", subs)
	}
	if subs[1].ID != "862" || subs[1].Name != "CS2" {
		t.Errorf("数字 gid 未归一: %+v", subs[1])
	}
	items := mapCategoryRows("live:huya:categories", 7, subs)
	if len(items) != 2 || items[0].ID != "huya-game:1" || items[0].Kind != platform.KindCategory {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Summary == nil || *items[0].Summary != "网游" {
		t.Errorf("summary = %v", items[0].Summary)
	}
	if items[0].CategoryChannel == nil || *items[0].CategoryChannel != "live:huya:category" {
		t.Errorf("categoryChannel = %v", items[0].CategoryChannel)
	}
	if items[0].CategoryInfo["gameId"] != "1" || items[0].FetchedAt != 7 {
		t.Errorf("categoryInfo/fetchedAt = %+v / %d", items[0].CategoryInfo, items[0].FetchedAt)
	}
}

func loadSearch(t *testing.T, name string) *searchResponse {
	t.Helper()
	var res searchResponse
	if err := platform.UnmarshalJSON(fixture(t, name), &res); err != nil {
		t.Fatal(err)
	}
	return &res
}

func TestMapAnchorRows(t *testing.T) {
	rows := loadSearch(t, "search_anchor.json").bucket("1")
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	items := make([]platform.Item, 0, len(rows))
	for _, row := range rows {
		if item := mapAnchorRow(row, 5); item != nil {
			items = append(items, *item)
		}
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	first := items[0]
	if first.ID != "huya:520000" || first.Title != "主播甲" || first.Platform != "huya" {
		t.Errorf("first = %+v", first)
	}
	if first.LiveStatus == nil || *first.LiveStatus != platform.LiveStatusLive {
		t.Errorf("first liveStatus = %v", first.LiveStatus)
	}
	if first.Thumbnail == nil || *first.Thumbnail != "https://avatar.example/a.jpg" {
		t.Errorf("first thumbnail = %v", first.Thumbnail)
	}
	if first.Author == nil || first.Author.Name != "主播甲" {
		t.Errorf("first author = %+v", first.Author)
	}
	if first.Online != nil {
		t.Errorf("主播桶不应输出 online: %v", first.Online)
	}
	raw, _ := json.Marshal(first)
	if strings.Contains(string(raw), `"online"`) {
		t.Errorf("json 不应含 online: %s", raw)
	}

	third := items[1]
	if third.ID != "huya:520001" || third.Title != "" || third.Author != nil || third.Thumbnail != nil {
		t.Errorf("third = %+v", third)
	}
	if third.LiveStatus == nil || *third.LiveStatus != platform.LiveStatusLive {
		t.Errorf("字符串 \"true\" 应视为在播: %v", third.LiveStatus)
	}
}

// 真实响应除房间/主播桶外还带其他内容桶(如 1026),值是裸数组甚至字符串;
// 曾经让整个响应 unmarshal 失败、搜索页该平台整组报错。
func TestSearchResponseToleratesRawBuckets(t *testing.T) {
	raw := platform.RawJSON(`{"response":{
		"1": {"docs": [{"room_id": 520000, "game_nick": "主播甲", "gameLiveOn": true}]},
		"3": {"docs": [{"room_id": 777, "game_introduction": "标题A"}]},
		"1026": [{"title": "相关视频"}, {"title": "相关视频2"}],
		"1027": "未知形态"
	}}`)
	var res searchResponse
	if err := platform.UnmarshalJSON(raw, &res); err != nil {
		t.Fatal(err)
	}
	if rows := res.bucket("1"); len(rows) != 1 || strVal(rows[0].GameNick) != "主播甲" {
		t.Errorf("bucket(1) = %+v", rows)
	}
	if rows := res.bucket("3"); len(rows) != 1 || strVal(rows[0].GameIntroduction) != "标题A" {
		t.Errorf("bucket(3) = %+v", rows)
	}
	if rows := res.bucket("1026"); rows != nil {
		t.Errorf("非预期数组桶应视为空: %+v", rows)
	}
	if rows := res.bucket("1027"); rows != nil {
		t.Errorf("未知形态桶应视为空: %+v", rows)
	}
	if rows := res.bucket("404"); rows != nil {
		t.Errorf("缺失桶应为空: %+v", rows)
	}
}

func TestMapSearchRooms(t *testing.T) {
	res := loadSearch(t, "search_rooms.json")
	rooms, anchors := res.bucket("3"), res.bucket("1")
	if len(rooms) != 2 || len(anchors) != 1 {
		t.Fatalf("rooms/anchors = %d / %d", len(rooms), len(anchors))
	}
	first := mapSearchRoom(rooms[0], anchors, 11)
	if first == nil || first.RoomID == nil || *first.RoomID != "777" {
		t.Fatalf("room_id 未用主播桶修正: %+v", first)
	}
	if first.ID != "huya:777" || first.Title != "标题A" || first.Online == nil || *first.Online != 321 {
		t.Errorf("first = %+v", first)
	}
	if first.Thumbnail == nil || *first.Thumbnail != "https://s.example/a.jpg?x-oss-process=style/w338_h190" {
		t.Errorf("first thumbnail = %v", first.Thumbnail)
	}
	second := mapSearchRoom(rooms[1], anchors, 11)
	if second == nil || *second.RoomID != "2" || second.Title != "房间B" || *second.Online != 5 {
		t.Errorf("second = %+v", second)
	}
	if second.Author != nil || second.Thumbnail != nil {
		t.Errorf("second author/thumbnail = %+v / %v", second.Author, second.Thumbnail)
	}
}
