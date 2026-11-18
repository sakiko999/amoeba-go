// 上游解析与 Item 映射测试:真实 API fixture(与 TS 黄金集同源)+ 边界用例。

package apple

import (
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestParseChartFixture(t *testing.T) {
	picks, err := parseChart(fixture(t, "chart_cn_genre26_limit3.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"1455784513", "1668626930", "1582119137"}
	if len(picks) != len(wantIDs) {
		t.Fatalf("picks = %d", len(picks))
	}
	for index, wantID := range wantIDs {
		if picks[index].id != wantID {
			t.Errorf("picks[%d].id = %q, want %q", index, picks[index].id, wantID)
		}
	}
	artwork := pickArtwork(picks[0].entry)
	if artwork == nil || !strings.HasSuffix(*artwork, "/170x170bb.png") {
		t.Errorf("pickArtwork = %v", artwork)
	}
	if genre := pickGenre(picks[0].entry); genre == nil || *genre != "喜剧" {
		t.Errorf("pickGenre = %v", genre)
	}
}

func TestParseLookupFixture(t *testing.T) {
	podcasts, err := parseLookup(fixture(t, "lookup_cn_3.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(podcasts) != 3 {
		t.Fatalf("podcasts = %d", len(podcasts))
	}
	first := podcasts[0]
	if first.ID != 1455784513 || first.Title != "凹凸电波" || first.Author != "凹凸电波" {
		t.Errorf("podcasts[0] = %+v", first)
	}
	if first.FeedURL != "http://www.ximalaya.com/album/19206382.xml" {
		t.Errorf("feedUrl = %q", first.FeedURL)
	}
	if first.Artwork == nil || !strings.Contains(*first.Artwork, "600x600bb") {
		t.Errorf("artwork = %v", first.Artwork)
	}
	if first.PageURL == nil || !strings.Contains(*first.PageURL, "id1455784513") {
		t.Errorf("pageUrl = %v", first.PageURL)
	}
	if first.Genre == nil || *first.Genre != "喜剧" || first.EpisodeCount == nil || *first.EpisodeCount != 395 {
		t.Errorf("genre/episodes = %v / %v", first.Genre, first.EpisodeCount)
	}
}

func TestMergeChartPreservesChartOrder(t *testing.T) {
	entries, err := parseChart(fixture(t, "chart_cn_genre26_limit3.json"))
	if err != nil {
		t.Fatal(err)
	}
	found, err := parseLookup(fixture(t, "lookup_cn_3.json"))
	if err != nil {
		t.Fatal(err)
	}
	reversed := []podcast{found[2], found[1], found[0]}
	merged := mergeChart(entries, reversed)
	if len(merged) != 3 {
		t.Fatalf("merged = %d", len(merged))
	}
	wantTitles := []string{"凹凸电波", "霓达播客", "岩中花述"}
	for index, wantTitle := range wantTitles {
		if merged[index].Title != wantTitle {
			t.Errorf("merged[%d] = %q, want %q", index, merged[index].Title, wantTitle)
		}
	}
}

func TestMergeChartFallsBackToChartFields(t *testing.T) {
	entries, err := parseChart(platform.RawJSON(`{"feed":{"entry":[
	  {"id":{"attributes":{"im:id":"42"}},
	   "im:image":[{"label":"small.jpg","attributes":{"height":"55"}},{"label":"large.jpg","attributes":{"height":"170"}}],
	   "category":{"attributes":{"label":"科技"}}},
	  {"id":{"attributes":{"im:id":"43"}},
	   "im:image":[{"label":"chart.jpg","attributes":{"height":"170"}}],
	   "category":{"attributes":{"label":"榜单分类"}}}
	]}}`))
	if err != nil {
		t.Fatal(err)
	}
	found, err := parseLookup(platform.RawJSON(`{"results":[
	  {"collectionId":43,"collectionName":"B","artistName":"b","feedUrl":"https://b/feed.xml","artworkUrl100":"small-100.jpg"},
	  {"collectionId":42,"collectionName":"A","artistName":"a","feedUrl":"https://a/feed.xml"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	merged := mergeChart(entries, found)
	if len(merged) != 2 {
		t.Fatalf("merged = %d", len(merged))
	}
	if merged[0].ID != 42 || merged[0].Artwork == nil || *merged[0].Artwork != "large.jpg" {
		t.Errorf("fallback artwork = %+v", merged[0])
	}
	if merged[0].Genre == nil || *merged[0].Genre != "科技" {
		t.Errorf("fallback genre = %v", merged[0].Genre)
	}
	if merged[1].Artwork == nil || *merged[1].Artwork != "small-100.jpg" {
		t.Errorf("lookup artwork should win: %v", merged[1].Artwork)
	}
	if merged[1].Genre == nil || *merged[1].Genre != "榜单分类" {
		t.Errorf("fallback genre = %v", merged[1].Genre)
	}
}

func TestToPodcastDropsIncomplete(t *testing.T) {
	cases := []struct {
		name   string
		result lookupResult
		wantOK bool
	}{
		{"no feedUrl", lookupResult{CollectionID: platform.Int64(1)}, false},
		{"blank feedUrl", lookupResult{CollectionID: platform.Int64(1), FeedURL: platform.Str("  ")}, false},
		{"no collectionId", lookupResult{FeedURL: platform.Str("https://feed")}, false},
		{"zero collectionId", lookupResult{CollectionID: platform.Int64(0), FeedURL: platform.Str("https://feed")}, false},
		{"valid", lookupResult{CollectionID: platform.Int64(7), FeedURL: platform.Str(" https://feed "), CollectionName: platform.Str(" T "), ArtistName: platform.Str(" A ")}, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			p, ok := toPodcast(testCase.result)
			if ok != testCase.wantOK {
				t.Fatalf("ok = %v, want %v", ok, testCase.wantOK)
			}
			if ok && (p.FeedURL != "https://feed" || p.Title != "T" || p.Author != "A") {
				t.Errorf("podcast = %+v", p)
			}
		})
	}
}

func samplePodcastItem(t *testing.T) map[string]any {
	t.Helper()
	podcasts := []podcast{{
		ID:           1455784513,
		Title:        "凹凸电波",
		Author:       "凹凸电波",
		FeedURL:      "https://feed.example/1.xml",
		Artwork:      platform.Str("https://img.example/600.jpg"),
		Genre:        platform.Str("喜剧"),
		EpisodeCount: platform.Int(395),
	}}
	items, err := podcastItems(podcasts, chartChannelKey, true, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	return shape
}

func TestPodcastItemsJSONShape(t *testing.T) {
	shape := samplePodcastItem(t)
	want := map[string]any{
		"id":        "apple-podcast-1455784513",
		"sourceId":  chartChannelKey,
		"kind":      "audio",
		"title":     "凹凸电波",
		"fetchedAt": float64(fixedNow),
		"url":       "https://feed.example/1.xml",
		"thumbnail": "https://img.example/600.jpg",
		"summary":   "凹凸电波 · 喜剧 · 395 集",
		"artist":    "凹凸电波",
	}
	for key, value := range want {
		if shape[key] != value {
			t.Errorf("shape[%s] = %v, want %v", key, shape[key], value)
		}
	}
}

func TestPodcastItemsNestedAndAbsent(t *testing.T) {
	shape := samplePodcastItem(t)
	for _, absent := range []string{"duration", "durationSec", "stream", "channel", "publishedAt", "content", "images", "likes"} {
		if _, ok := shape[absent]; ok {
			t.Errorf("unexpected optional field %q", absent)
		}
	}
	author, ok := shape["author"].(map[string]any)
	if !ok || author["name"] != "凹凸电波" {
		t.Errorf("author = %v", shape["author"])
	}
	itemRaw, ok := shape["raw"].(map[string]any)
	if !ok || itemRaw["rank"] != float64(1) || itemRaw["appleId"] != float64(1455784513) {
		t.Errorf("raw = %v", shape["raw"])
	}
}

func TestPodcastItemsOmitEmptyOptionals(t *testing.T) {
	items, err := podcastItems([]podcast{{ID: 2, Title: "T", FeedURL: "https://feed"}}, searchChannelKey, false, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if shape["summary"] != "" {
		t.Errorf("summary = %v, want 空串(TS 模板串总会输出)", shape["summary"])
	}
	for _, absent := range []string{"thumbnail", "author", "artist"} {
		if _, ok := shape[absent]; ok {
			t.Errorf("unexpected field %q in %s", absent, raw)
		}
	}
	itemRaw := shape["raw"].(map[string]any)
	if _, hasRank := itemRaw["rank"]; hasRank {
		t.Errorf("raw.rank 应省略: %v", itemRaw)
	}
	if _, hasPage := itemRaw["pageUrl"]; hasPage {
		t.Errorf("raw.pageUrl 应省略: %v", itemRaw)
	}
}
