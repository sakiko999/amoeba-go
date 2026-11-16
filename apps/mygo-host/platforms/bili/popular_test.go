// 移植 TS example/bili-popular.check.ts：parsePopularItems 的字段映射黄金样本。

package bili

import (
	"encoding/json"
	"os"
	"testing"
)

func TestParsePopularItems(t *testing.T) {
	payloadJSON, err := os.ReadFile("testdata/popular.json")
	if err != nil {
		t.Fatal(err)
	}
	var payload popularPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	items := parsePopularItems(&payload, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.Kind != "video" || first.SourceID != "bili:popular" {
		t.Errorf("kind/sourceId = %v/%v", first.Kind, first.SourceID)
	}
	if first.ID != "BV1xx411c7mD" || first.URL == nil || *first.URL != "https://www.bilibili.com/video/BV1xx411c7mD" {
		t.Errorf("id/url = %q/%v", first.ID, first.URL)
	}
	if first.Title != "示例视频一" {
		t.Errorf("title = %q", first.Title)
	}
	if first.Summary == nil || *first.Summary != "简介一" {
		t.Errorf("summary = %v", first.Summary)
	}
	if first.Thumbnail == nil || *first.Thumbnail != "https://i0.hdslb.com/cover1.jpg" ||
		first.Poster == nil || *first.Poster != "https://i0.hdslb.com/cover1.jpg" {
		t.Errorf("thumbnail/poster = %v/%v", first.Thumbnail, first.Poster)
	}
	if first.Author == nil || first.Author.Name != "UP主甲" || first.Channel == nil || first.Channel.Name != "UP主甲" {
		t.Errorf("author/channel = %+v/%+v", first.Author, first.Channel)
	}
	if first.PublishedAt == nil || *first.PublishedAt != 1_700_000_000_000 {
		t.Errorf("publishedAt = %v", first.PublishedAt)
	}
	if first.Duration == nil || *first.Duration != 100 {
		t.Errorf("duration = %v", first.Duration)
	}
	if first.FetchedAt != testNow {
		t.Errorf("fetchedAt = %d", first.FetchedAt)
	}

	second := items[1]
	if second.ID != "av789" || second.URL == nil || *second.URL != "https://www.bilibili.com/video/av789" {
		t.Errorf("无 bvid 回落 = %q/%v", second.ID, second.URL)
	}
	if second.PublishedAt != nil {
		t.Errorf("无 pubdate 应留空: %v", second.PublishedAt)
	}
	if second.Thumbnail != nil || second.Author != nil {
		t.Error("缺字段不应伪造 thumbnail/author")
	}

	if len(parsePopularItems(nil, testNow)) != 0 {
		t.Error("nil payload 应为 0 条")
	}
	if len(parsePopularItems(&popularPayload{}, testNow)) != 0 {
		t.Error("无 data 应为 0 条")
	}
}
