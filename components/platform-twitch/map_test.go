// Live 映射测试(移植 TS example/twitch-live.check.ts 的离线黄金集):
// twitch:live 的 live/offline 两态与标题回落,twitch:hot/category 的流列表映射。

package twitch

import (
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

const fixedAt int64 = 1_700_000_000_000

func sp(s string) *string { return &s }
func ip(v int) *int       { return &v }

func checkStr(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", label, got, want)
	}
}

func checkStrPtr(t *testing.T, label string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %q", label, got, want)
	}
}

func checkNilStr(t *testing.T, label string, got *string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s = %q, want nil", label, *got)
	}
}

func checkIntPtr(t *testing.T, label string, got *int, want int) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %d", label, got, want)
	}
}

func sampleUser() *gqlUser {
	return &gqlUser{
		Login:       sp("shroud"),
		DisplayName: sp("Shroud"),
		Description: sp("FPS streamer"),
		Stream: &gqlStreamInfo{
			ID:           sp("4132001"),
			Title:        sp("Ranked grind"),
			ViewersCount: ip(12345),
			CreatedAt:    sp("2026-10-20T08:00:00.000Z"),
			Type:         sp("live"),
			Game:         &gqlGameName{Name: sp("VALORANT")},
		},
	}
}

func sampleStreamNode() gqlStreamNode {
	return gqlStreamNode{
		ID:              sp("4132001"),
		Title:           sp("Ranked grind"),
		ViewersCount:    ip(12345),
		CreatedAt:       sp("2026-10-20T08:00:00.000Z"),
		Broadcaster:     &gqlBroadcaster{Login: sp("Shroud"), DisplayName: sp("Shroud")},
		Game:            &gqlGameSlug{Name: sp("VALORANT"), Slug: sp("valorant")},
		PreviewImageURL: sp("https://static-cdn.jtvnw.net/previews-ttv/live_user_shroud-640x360.jpg"),
	}
}

func TestTwitchLiveItemLive(t *testing.T) {
	item := twitchLiveItem(sampleUser(), "shroud", fixedAt)
	checkStr(t, "id", item.ID, "twitch:shroud")
	checkStr(t, "sourceId", item.SourceID, "twitch:live")
	if item.Kind != platform.KindLive {
		t.Errorf("kind = %s", item.Kind)
	}
	checkStrPtr(t, "url", item.URL, "https://www.twitch.tv/shroud")
	checkStr(t, "platform", item.Platform, "twitch")
	checkStrPtr(t, "roomId", item.RoomID, "shroud")
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	checkIntPtr(t, "online", item.Online, 12345)
	checkStr(t, "title", item.Title, "Ranked grind")
	checkStrPtr(t, "thumbnail", item.Thumbnail,
		"https://static-cdn.jtvnw.net/previews-ttv/live_user_shroud-640x360.jpg")
	if item.Author == nil {
		t.Fatal("author 为空")
	}
	checkStr(t, "author.name", item.Author.Name, "Shroud")
	checkStrPtr(t, "introduction", item.Introduction, "FPS streamer")
	checkStrPtr(t, "showTime", item.ShowTime, "2026-10-20T08:00:00.000Z")
	if item.FetchedAt != fixedAt {
		t.Errorf("fetchedAt = %d", item.FetchedAt)
	}
}

func TestTwitchLiveItemOffline(t *testing.T) {
	user := &gqlUser{Login: sp("some_streamer"), DisplayName: sp("SomeStreamer"), Description: sp("")}
	item := twitchLiveItem(user, "some_streamer", fixedAt)
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusOffline {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	checkIntPtr(t, "online", item.Online, 0)
	checkStr(t, "title", item.Title, "SomeStreamer（离线）")
	checkNilStr(t, "showTime", item.ShowTime)
	checkNilStr(t, "introduction", item.Introduction)
	checkStr(t, "id", item.ID, "twitch:some_streamer")
}

func TestTwitchLiveItemTitleFallbacks(t *testing.T) {
	emptyTitle := sampleUser()
	emptyTitle.Stream.Title = sp("")
	item := twitchLiveItem(emptyTitle, "empty_title", fixedAt)
	checkStr(t, "空标题 title", item.Title, "Shroud")
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("空标题 liveStatus = %v", item.LiveStatus)
	}

	badTime := sampleUser()
	badTime.Stream.CreatedAt = sp("not-a-date")
	item = twitchLiveItem(badTime, "bad_time", fixedAt)
	checkNilStr(t, "坏 createdAt showTime", item.ShowTime)
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("坏 createdAt liveStatus = %v", item.LiveStatus)
	}

	noName := sampleUser()
	noName.DisplayName = sp("")
	noName.Stream.Title = sp("")
	item = twitchLiveItem(noName, "no_name", fixedAt)
	checkStr(t, "空 displayName title", item.Title, "no_name")
	checkStr(t, "空 displayName author", item.Author.Name, "no_name")
}

func TestTwitchStreamToLive(t *testing.T) {
	item := twitchStreamToLive(sampleStreamNode(), "twitch:hot", fixedAt)
	if item == nil {
		t.Fatal("正常流不应为 nil")
	}
	checkStr(t, "id", item.ID, "twitch:shroud")
	checkStr(t, "sourceId", item.SourceID, "twitch:hot")
	if item.Kind != platform.KindLive {
		t.Errorf("kind = %s", item.Kind)
	}
	checkStrPtr(t, "url", item.URL, "https://www.twitch.tv/shroud")
	checkStr(t, "platform", item.Platform, "twitch")
	checkStrPtr(t, "roomId", item.RoomID, "shroud")
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	checkIntPtr(t, "online", item.Online, 12345)
	checkStr(t, "title", item.Title, "Ranked grind")
	checkStrPtr(t, "thumbnail", item.Thumbnail,
		"https://static-cdn.jtvnw.net/previews-ttv/live_user_shroud-640x360.jpg")
	checkStr(t, "author.name", item.Author.Name, "Shroud")
	checkStrPtr(t, "showTime", item.ShowTime, "2026-10-20T08:00:00.000Z")
	if item.FetchedAt != fixedAt {
		t.Errorf("fetchedAt = %d", item.FetchedAt)
	}
}

func TestTwitchStreamToLiveFallbacks(t *testing.T) {
	emptyPreview := sampleStreamNode()
	emptyPreview.Broadcaster = &gqlBroadcaster{Login: sp("some_streamer"), DisplayName: sp("SomeStreamer")}
	emptyPreview.PreviewImageURL = sp("")
	item := twitchStreamToLive(emptyPreview, "twitch:category", fixedAt)
	checkStrPtr(t, "空 previewImageURL thumbnail", item.Thumbnail,
		"https://static-cdn.jtvnw.net/previews-ttv/live_user_some_streamer-640x360.jpg")
	checkStr(t, "sourceId", item.SourceID, "twitch:category")
	checkStr(t, "title", item.Title, "Ranked grind")

	nullPreview := sampleStreamNode()
	nullPreview.PreviewImageURL = nil
	item = twitchStreamToLive(nullPreview, "twitch:hot", fixedAt)
	checkStrPtr(t, "null previewImageURL thumbnail", item.Thumbnail,
		"https://static-cdn.jtvnw.net/previews-ttv/live_user_shroud-640x360.jpg")

	blankLogin := sampleStreamNode()
	blankLogin.Broadcaster = &gqlBroadcaster{Login: sp("   "), DisplayName: sp("Ghost")}
	if item := twitchStreamToLive(blankLogin, "twitch:hot", fixedAt); item != nil {
		t.Errorf("空 broadcaster.login = %+v, want nil", item)
	}
	missingLogin := sampleStreamNode()
	missingLogin.Broadcaster = &gqlBroadcaster{DisplayName: sp("Ghost")}
	if item := twitchStreamToLive(missingLogin, "twitch:hot", fixedAt); item != nil {
		t.Errorf("缺失 broadcaster.login = %+v, want nil", item)
	}

	badTime := sampleStreamNode()
	badTime.CreatedAt = sp("not-a-date")
	item = twitchStreamToLive(badTime, "twitch:hot", fixedAt)
	checkNilStr(t, "坏 createdAt showTime", item.ShowTime)
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("坏 createdAt liveStatus = %v", item.LiveStatus)
	}

	emptyTitle := sampleStreamNode()
	emptyTitle.Title = sp("")
	emptyTitle.Broadcaster = &gqlBroadcaster{Login: sp("empty_title"), DisplayName: sp("EmptyTitle")}
	item = twitchStreamToLive(emptyTitle, "twitch:hot", fixedAt)
	checkStr(t, "空标题 title", item.Title, "EmptyTitle")

	noName := sampleStreamNode()
	noName.Title = sp("")
	noName.Broadcaster = &gqlBroadcaster{Login: sp("no_name"), DisplayName: sp("")}
	item = twitchStreamToLive(noName, "twitch:hot", fixedAt)
	checkStr(t, "空 displayName title", item.Title, "no_name")
	checkStr(t, "空 displayName author", item.Author.Name, "no_name")
}

func TestLiveItemJSONShape(t *testing.T) {
	raw, err := json.Marshal(twitchLiveItem(sampleUser(), "shroud", fixedAt))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"id":"twitch:shroud"`, `"sourceId":"twitch:live"`, `"kind":"live"`,
		`"title":"Ranked grind"`, `"fetchedAt":1700000000000`,
		`"url":"https://www.twitch.tv/shroud"`, `"roomId":"shroud"`,
		`"liveStatus":"live"`, `"online":12345`, `"platform":"twitch"`,
		`"showTime":"2026-10-20T08:00:00.000Z"`, `"introduction":"FPS streamer"`,
		`"author":{"name":"Shroud"}`,
	} {
		if !strings.Contains(string(raw), fragment) {
			t.Errorf("JSON 缺少 %s: %s", fragment, raw)
		}
	}

	offline, err := json.Marshal(twitchLiveItem(&gqlUser{DisplayName: sp("SomeStreamer")}, "some_streamer", fixedAt))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(offline), "showTime") || strings.Contains(string(offline), "introduction") {
		t.Errorf("离线条目不应输出 showTime/introduction: %s", offline)
	}
	if !strings.Contains(string(offline), `"title":"SomeStreamer（离线）"`) {
		t.Errorf("离线 title = %s", offline)
	}
}

func TestCategoryItem(t *testing.T) {
	game := gqlCategoryNode{
		Name:         sp("Just Chatting"),
		Slug:         sp("just-chatting"),
		ViewersCount: ip(1234),
		BoxArtURL:    sp("https://example/box.jpg"),
	}
	item := categoryItem(game, fixedAt)
	checkStr(t, "id", item.ID, "twitch-category:just-chatting")
	checkStr(t, "sourceId", item.SourceID, "twitch:categories")
	if item.Kind != platform.KindCategory {
		t.Errorf("kind = %s", item.Kind)
	}
	checkStr(t, "title", item.Title, "Just Chatting")
	checkStrPtr(t, "url", item.URL, "https://www.twitch.tv/directory/category/just-chatting")
	checkStrPtr(t, "summary", item.Summary, "1234 人在看")
	checkStrPtr(t, "thumbnail", item.Thumbnail, "https://example/box.jpg")
	checkStrPtr(t, "categoryChannel", item.CategoryChannel, "twitch:category")
	checkStr(t, "categoryInfo", item.CategoryInfo["category"], "just-chatting")

	empty := categoryItem(gqlCategoryNode{Slug: sp("fallback")}, fixedAt)
	checkStr(t, "空名称 title 回落 slug", empty.Title, "fallback")
	checkNilStr(t, "0 观众 summary", empty.Summary)
}

func TestSearchItem(t *testing.T) {
	viewers := 4321
	item := searchItem(twitchChannelHit{
		Login: "shroud", DisplayName: "Shroud", Description: "FPS streamer", Followers: 12345,
		Avatar: "https://img.example/shroud-70.png",
		Stream: &gqlStreamInfo{
			Title:           new("SEARCHING"),
			ViewersCount:    &viewers,
			CreatedAt:       new("2026-01-02T03:04:05Z"),
			PreviewImageURL: new("https://img.example/live.jpg"),
		},
	}, fixedAt)
	checkStr(t, "id", item.ID, "tw-shroud")
	checkStr(t, "sourceId", item.SourceID, "twitch:user:search")
	if item.Kind != platform.KindLive {
		t.Errorf("kind = %s", item.Kind)
	}
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	checkStr(t, "title", item.Title, "SEARCHING")
	checkStrPtr(t, "url", item.URL, "https://www.twitch.tv/shroud")
	checkStrPtr(t, "summary", item.Summary, "1.2万 关注者 · FPS streamer")
	checkStrPtr(t, "thumbnail", item.Thumbnail, "https://img.example/live.jpg")
	if item.Author == nil {
		t.Fatal("author 缺失")
	}
	checkStr(t, "author name", item.Author.Name, "Shroud")
	checkStrPtr(t, "author avatar", item.Author.Avatar, "https://img.example/shroud-70.png")
	checkStrPtr(t, "roomId", item.RoomID, "shroud")
	if item.Online == nil || *item.Online != 4321 {
		t.Errorf("online = %v", item.Online)
	}

	offline := searchItem(twitchChannelHit{
		Login: "x", DisplayName: "X", Avatar: "https://img.example/x.png",
	}, fixedAt)
	if offline.LiveStatus == nil || *offline.LiveStatus != platform.LiveStatusOffline {
		t.Errorf("离线 liveStatus = %v", offline.LiveStatus)
	}
	checkStr(t, "离线 title", offline.Title, "X")
	checkStrPtr(t, "离线 thumbnail", offline.Thumbnail, "https://img.example/x.png")
	checkNilStr(t, "无关注/描述 summary", offline.Summary)

	long := searchItem(twitchChannelHit{Login: "x", DisplayName: "X", Description: "abcdefghij"}, fixedAt)
	if long.Summary == nil || *long.Summary != "abcdefghij" {
		t.Errorf("短描述 summary = %v", long.Summary)
	}
}

func TestFmtCount(t *testing.T) {
	cases := map[int]string{
		0: "0", 9999: "9999", 10000: "1万", 12345: "1.2万",
		12500: "1.3万", 14500: "1.4万", 17500: "1.8万", 22500: "2.3万", 34500: "3.5万",
		100000000: "1亿", 125000000: "1.3亿", 340000000: "3.4亿",
	}
	for input, want := range cases {
		if got := fmtCount(input); got != want {
			t.Errorf("fmtCount(%d) = %q, want %q", input, got, want)
		}
	}
}
