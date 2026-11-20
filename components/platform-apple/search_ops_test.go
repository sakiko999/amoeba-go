// apple entity 搜索 op 测试:scope 校验、请求参数与节目页映射。

package apple

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"mygo-host/platform"
)

func TestSearchOpEntity(t *testing.T) {
	client, urls := fakeCatalogClient(t, map[string]platform.RawJSON{
		"/search": fixture(t, "search_cn_koutu.json"),
	})
	ops := New(testContext(t, client)).Ops
	raw, err := ops["search"].Invoke(context.Background(),
		json.RawMessage(`{"keyword":" 凹凸电波 ","scope":"entity","pageSize":3}`))
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://itunes.apple.com/search?term=%E5%87%B9%E5%87%B8%E7%94%B5%E6%B3%A2&media=podcast&entity=podcast&limit=3&country=cn"
	if !reflect.DeepEqual(*urls, []string{wantURL}) {
		t.Fatalf("request urls = %v", *urls)
	}
	page := raw.(platform.FetchPage)
	if len(page.Items) != 1 {
		t.Fatalf("items = %d", len(page.Items))
	}
	item := page.Items[0]
	if item.SourceID != searchChannelKey || item.Kind != platform.KindAudio {
		t.Errorf("item source/kind = %s / %s", item.SourceID, item.Kind)
	}
	if item.URL == nil || *item.URL != "https://podcasts.apple.com/cn/podcast/%E5%87%B9%E5%87%B8%E7%94%B5%E6%B3%A2/id1455784513?uo=4" {
		t.Errorf("item.url = %v", item.URL)
	}
	if item.Author == nil || item.Author.Name != "凹凸电波" || item.Summary == nil || *item.Summary != "凹凸电波 · 喜剧 · 395 集" {
		t.Errorf("item author/summary = %+v / %v", item.Author, item.Summary)
	}
	var itemRaw entityRaw
	if err := json.Unmarshal(item.Raw, &itemRaw); err != nil {
		t.Fatal(err)
	}
	if itemRaw.FeedURL != "http://www.ximalaya.com/album/19206382.xml" || itemRaw.PageURL != *item.URL {
		t.Errorf("item.raw = %+v", itemRaw)
	}
}

func TestSearchOpRejectsLive(t *testing.T) {
	op := New(testContext(t, nil)).Ops["search"]
	_, err := op.Invoke(context.Background(), json.RawMessage(`{"keyword":"x","scope":"live"}`))
	if err == nil {
		t.Fatal("live scope 不应被 apple 支持")
	}
}
