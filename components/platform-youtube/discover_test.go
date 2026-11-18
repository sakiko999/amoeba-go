// 直播页解析/栏目 slug/playlistId 归一测试(移植 src/example/youtube-channels.check.ts)。

package youtube

import "testing"

func liveVideoEntry(id, title, viewers string, live bool, owner string) map[string]any {
	var overlay map[string]any
	if live {
		overlay = map[string]any{"thumbnailOverlayTimeStatusRenderer": map[string]any{
			"text": map[string]any{"runs": []any{map[string]any{"text": "LIVE"}}},
		}}
	} else {
		overlay = map[string]any{"thumbnailOverlayTimeStatusRenderer": map[string]any{
			"text": map[string]any{"simpleText": "57:17"},
		}}
	}
	renderer := map[string]any{
		"videoId":           id,
		"title":             map[string]any{"runs": []any{map[string]any{"text": title}}},
		"viewCountText":     map[string]any{"simpleText": viewers},
		"thumbnail":         map[string]any{"thumbnails": []any{map[string]any{"url": "https://i.ytimg.com/vi/" + id + "/hqdefault.jpg", "width": 480}}},
		"thumbnailOverlays": []any{overlay},
	}
	if owner != "" {
		renderer["ownerText"] = map[string]any{"runs": []any{map[string]any{"text": owner}}}
	}
	return map[string]any{"richItemRenderer": map[string]any{"content": map[string]any{"videoRenderer": renderer}}}
}

func liveShelfEntry(title any, items ...any) map[string]any {
	return map[string]any{"richSectionRenderer": map[string]any{"content": map[string]any{
		"richShelfRenderer": map[string]any{"title": title, "contents": items},
	}}}
}

func TestYoutubeShelfSlug(t *testing.T) {
	cases := map[string]string{
		"Live Now":            "hot",
		"Live Now - Gaming":   "gaming",
		"Live Now - News":     "news",
		"Recent Live Streams": "recent-live-streams",
	}
	for input, want := range cases {
		if got := youtubeShelfSlug(input); got != want {
			t.Errorf("youtubeShelfSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeYoutubeLiveCategory(t *testing.T) {
	cases := map[string]string{
		"Gaming":            "gaming",
		"Live Now - Sports": "sports",
		"gaming!":           "gaming",
		"":                  "",
		"a":                 "",
	}
	for input, want := range cases {
		if got := normalizeYoutubeLiveCategory(input); got != want {
			t.Errorf("normalizeYoutubeLiveCategory(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseYoutubeLiveShelves(t *testing.T) {
	page := map[string]any{"contents": map[string]any{"twoColumnBrowseResultsRenderer": map[string]any{
		"tabs": []any{map[string]any{"tabRenderer": map[string]any{"content": map[string]any{
			"richGridRenderer": map[string]any{"contents": []any{
				// 顶部轮播:没有 richShelfRenderer / 标题,跳过
				map[string]any{"richSectionRenderer": map[string]any{"content": map[string]any{"carouselItemRenderer": map[string]any{}}}},
				liveShelfEntry(
					map[string]any{"runs": []any{map[string]any{"text": "Live Now"}}},
					liveVideoEntry("aaa", "Live A", "19,926 watching", true, "Channel"),
					liveVideoEntry("bbb", "Replay B", "25,198,023 views", false, ""),
				),
				liveShelfEntry(
					map[string]any{"simpleText": "Live Now - News"},
					liveVideoEntry("ccc", "News C", "2,472 watching", true, ""),
					liveVideoEntry("ddd", "Upcoming D", "5 waiting", false, ""),
				),
				// 旧布局兼容:直接 richShelfRenderer
				map[string]any{"richShelfRenderer": map[string]any{
					"title":    map[string]any{"simpleText": "Live Now - Sports"},
					"contents": []any{liveVideoEntry("eee", "Sport E", "740 watching", true, "")},
				}},
			}},
		}}}},
	}}}

	shelves := parseYoutubeLiveShelves(mustMarshal(t, page))
	if len(shelves) != 3 {
		t.Fatalf("栏目数 = %d", len(shelves))
	}
	bySlug := map[string]youtubeLiveShelf{}
	for _, shelf := range shelves {
		bySlug[shelf.Slug] = shelf
	}
	hot, ok := bySlug["hot"]
	if !ok || hot.Title != "Live Now" || len(hot.Hits) != 1 {
		t.Fatalf("hot = %+v", hot)
	}
	hit := hot.Hits[0]
	if hit.VideoID != "aaa" || hit.Title != "Live A" || hit.ChannelTitle != "Channel" {
		t.Errorf("hot hit = %+v", hit)
	}
	if hit.Viewers == nil || *hit.Viewers != 19926 {
		t.Errorf("watching 解析 = %v", hit.Viewers)
	}
	news := bySlug["news"]
	if len(news.Hits) != 1 || news.Hits[0].VideoID != "ccc" {
		t.Errorf("news hits = %+v", news.Hits)
	}
	if news.Hits[0].Viewers == nil || *news.Hits[0].Viewers != 2472 {
		t.Errorf("news viewers = %v", news.Hits[0].Viewers)
	}
	if sports := bySlug["sports"]; len(sports.Hits) != 1 {
		t.Error("旧布局 richShelfRenderer 也应解析")
	}
	if empty := parseYoutubeLiveShelves(mustJSON(t, "{}")); len(empty) != 0 {
		t.Errorf("空响应应得空数组,得 %d", len(empty))
	}
}

func TestNormalizeYoutubePlaylistId(t *testing.T) {
	const playlistID = "PLFgquLnL59alCl_2TQvOiD5Vgm1hCaGSI"
	valid := []string{
		playlistID,
		"https://www.youtube.com/playlist?list=" + playlistID,
		"https://www.youtube.com/watch?v=abc12345678&list=" + playlistID,
		"youtu.be/abc12345678?list=" + playlistID,
		"https://music.youtube.com/playlist?list=" + playlistID,
	}
	for _, input := range valid {
		if got := normalizeYoutubePlaylistID(input); got != playlistID {
			t.Errorf("normalizeYoutubePlaylistID(%q) = %q", input, got)
		}
	}
	invalid := []string{
		"RDabc12345678", "WLabc12345678", "LLabc12345678", "LMabc12345678",
		"https://example.com/playlist?list=" + playlistID, "",
	}
	for _, input := range invalid {
		if got := normalizeYoutubePlaylistID(input); got != "" {
			t.Errorf("normalizeYoutubePlaylistID(%q) 应为空,得 %q", input, got)
		}
	}
}
