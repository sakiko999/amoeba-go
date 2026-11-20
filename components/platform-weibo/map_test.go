// 用户卡 → Item 映射黄金集(规格 §3.5 fixture)。

package weibo

import (
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

func fixtureItems(t *testing.T) []platform.Item {
	t.Helper()
	hits, found, _ := parseUserSearch([]platform.CapturedResponse{{
		URL:  capturedURL(userContainer("半佛仙人")),
		Body: fixture(t, "user_search.json"),
	}}, "半佛仙人")
	if !found {
		t.Fatal("fixture 未解析出 ok=1")
	}
	return userSearchItems(hits, testNow)
}

func TestUserSearchItemsMapping(t *testing.T) {
	items := fixtureItems(t)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.ID != "wbu-1234567890" || first.SourceID != "weibo:user:search" ||
		first.Kind != platform.KindVideo || first.Title != "半佛仙人" {
		t.Errorf("items[0] 基础字段 = %+v", first)
	}
	if str(first.URL) != "https://weibo.com/u/1234567890" {
		t.Errorf("url = %v", str(first.URL))
	}
	if str(first.Thumbnail) != "https://tvax1.sinaimg.cn/avatar_hd.jpg" {
		t.Errorf("thumbnail = %v, want avatar_hd 优先", str(first.Thumbnail))
	}
	if str(first.Summary) != "知名互联网资讯博主 · 207.7万 关注者" {
		t.Errorf("summary = %v", str(first.Summary))
	}
	if first.FetchedAt != testNow {
		t.Errorf("fetchedAt = %d", first.FetchedAt)
	}
	if first.Author == nil || first.Author.Name != "半佛仙人" || first.Author.Handle == nil ||
		*first.Author.Handle != "1234567890" || first.Author.Avatar == nil {
		t.Errorf("author = %+v", first.Author)
	}

	second := items[1]
	if second.ID != "wbu-987654321" || second.Title != "另一个用户" {
		t.Errorf("items[1] = %+v", second)
	}
	if str(second.Thumbnail) != "https://tvax2.sinaimg.cn/profile.jpg" {
		t.Errorf("thumbnail = %v, want profile_image_url 兜底", str(second.Thumbnail))
	}
	if str(second.Summary) != "34 关注者" {
		t.Errorf("summary = %v", str(second.Summary))
	}
}

func TestUserSearchItemsJSONShape(t *testing.T) {
	raw, err := json.Marshal(fixtureItems(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"id":"wbu-1234567890"`,
		`"sourceId":"weibo:user:search"`,
		`"kind":"video"`,
		`"url":"https://weibo.com/u/1234567890"`,
		`"author":{"name":"半佛仙人"`,
		`"handle":"1234567890"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("JSON 缺少 %s: %s", want, raw)
		}
	}
}

func str(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
