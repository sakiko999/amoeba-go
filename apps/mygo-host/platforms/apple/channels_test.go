// 渠道声明与 feed 协议出口的离线测试(对应 TS tests/platform.test.ts + 协议行为)。

package apple

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mygo-host/platform"
)

func channelByKey(t *testing.T, key string) platform.DataChannel {
	t.Helper()
	for _, channel := range channels(testContext(t, nil)) {
		if channel.Key() == key {
			return channel
		}
	}
	t.Fatalf("channel %q not found", key)
	return nil
}

func TestChannelDeclarations(t *testing.T) {
	list := channels(testContext(t, nil))
	if len(list) == 0 {
		t.Fatal("apple 没有声明任何渠道")
	}
	seen := map[string]bool{}
	for _, channel := range list {
		if channel.Name() == "" {
			t.Errorf("渠道 %q 缺少名称", channel.Key())
		}
		if seen[channel.Key()] {
			t.Errorf("渠道 key %q 重复", channel.Key())
		}
		seen[channel.Key()] = true
		if channel.Kind() != platform.KindAudio {
			t.Errorf("渠道 %q kind = %q", channel.Key(), channel.Kind())
		}
	}
}

func TestDefaultInfoChannelsInstantiate(t *testing.T) {
	withDefault := 0
	for _, channel := range channels(testContext(t, nil)) {
		defaults := channel.DefaultInfo()
		if defaults == nil {
			continue
		}
		withDefault++
		if source := channel.GetSource(cloneInfo(defaults)); source == nil {
			t.Errorf("渠道 %q 无法零参实例化", channel.Key())
		}
	}
	if withDefault != 1 {
		t.Errorf("声明 defaultInfo 的渠道数 = %d, want 1(chart)", withDefault)
	}
	if got := channelByKey(t, chartChannelKey).DefaultInfo()["genre"]; got != "26" {
		t.Errorf("chart defaultInfo.genre = %q", got)
	}
	if defaults := channelByKey(t, searchChannelKey).DefaultInfo(); defaults != nil {
		t.Errorf("search 刻意不声明 defaultInfo,got %v", defaults)
	}
}

func TestNoPassiveChannels(t *testing.T) {
	spec := New(testContext(t, nil))
	metas, err := spec.Ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, meta := range metas.([]platform.ChannelMeta) {
		if meta.Passive {
			t.Errorf("渠道 %q 不应声明 passive", meta.Key)
		}
	}
	_, err = spec.Ops["fetch"].Invoke(context.Background(),
		json.RawMessage(`{"channel":"apple:podcast-chart","passive":true}`))
	if err == nil || !strings.Contains(err.Error(), "不支持被动采集") {
		t.Errorf("passive fetch error = %v", err)
	}
}

func TestChannelsOpJSONMatchesTS(t *testing.T) {
	spec := New(testContext(t, nil))
	result, err := spec.Ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantChannelsJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("channels JSON 与 TS 不一致:\n got = %s\nwant = %s", raw, wantChannelsJSON)
	}
}

const wantChannelsJSON = `[
  {"key":"apple:podcast-chart","name":"Apple 播客热门榜","kind":"audio",
   "sourceInfoTpl":[
     {"key":"genre","label":"分类(26=全部 / 1324=社会与文化 / 1304=教育 / 1321=商业 / 1318=科技 / 1303=喜剧 / 1489=新闻 / 1512=健康与健身)","required":false},
     {"key":"country","label":"国家区(如 cn / us),默认 cn","required":false},
     {"key":"limit","label":"条数(默认 20,最多 200)","required":false}],
   "defaultInfo":{"genre":"26"}},
  {"key":"apple:podcast-search","name":"Apple 播客搜索","kind":"audio",
   "sourceInfoTpl":[
     {"key":"q","label":"关键词","required":true},
     {"key":"country","label":"国家区(如 cn / us),默认 cn","required":false},
     {"key":"limit","label":"条数(默认 20,最多 100)","required":false}]}
]`

func cloneInfo(info platform.SourceInfo) platform.SourceInfo {
	copied := make(platform.SourceInfo, len(info))
	for key, value := range info {
		copied[key] = value
	}
	return copied
}

func TestChartFetchRequests(t *testing.T) {
	_, urls := fetchChartResult(t)
	wantURLs := []string{
		"https://itunes.apple.com/us/rss/toppodcasts/limit=3/genre=1324/json",
		"https://itunes.apple.com/lookup?id=1455784513,1668626930,1582119137&entity=podcast&country=us",
	}
	if !reflect.DeepEqual(urls, wantURLs) {
		t.Errorf("request urls = %v, want %v", urls, wantURLs)
	}
}

func TestChartFetchItems(t *testing.T) {
	feed, _ := fetchChartResult(t)
	if len(feed.Items) != 3 {
		t.Fatalf("items = %d", len(feed.Items))
	}
	wantIDs := []string{"apple-podcast-1455784513", "apple-podcast-1668626930", "apple-podcast-1582119137"}
	for index, wantID := range wantIDs {
		if feed.Items[index].ID != wantID {
			t.Errorf("items[%d].id = %q, want %q", index, feed.Items[index].ID, wantID)
		}
	}
	assertFirstChartItem(t, feed.Items[0])
}

func assertFirstChartItem(t *testing.T, first platform.Item) {
	t.Helper()
	if first.SourceID != chartChannelKey || first.Kind != platform.KindAudio || first.Title != "凹凸电波" {
		t.Errorf("first = %+v", first)
	}
	if first.URL == nil || *first.URL != "http://www.ximalaya.com/album/19206382.xml" {
		t.Errorf("first.url = %v", first.URL)
	}
	if first.Summary == nil || *first.Summary != "凹凸电波 · 喜剧 · 395 集" {
		t.Errorf("first.summary = %v", first.Summary)
	}
	if first.Artist == nil || *first.Artist != "凹凸电波" || first.Author == nil || first.Author.Name != "凹凸电波" {
		t.Errorf("first author/artist = %+v / %v", first.Author, first.Artist)
	}
	if first.Thumbnail == nil || !strings.Contains(*first.Thumbnail, "600x600bb") {
		t.Errorf("first.thumbnail = %v", first.Thumbnail)
	}
	if first.FetchedAt != fixedNow {
		t.Errorf("first.fetchedAt = %d", first.FetchedAt)
	}
}

func TestChartFetchRaw(t *testing.T) {
	feed, _ := fetchChartResult(t)
	var raw map[string]any
	if err := json.Unmarshal(feed.Items[0].Raw, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["rank"] != float64(1) || raw["appleId"] != float64(1455784513) {
		t.Errorf("first.raw = %v", raw)
	}
	if raw["feedUrl"] != "http://www.ximalaya.com/album/19206382.xml" {
		t.Errorf("first.raw.feedUrl = %v", raw["feedUrl"])
	}
}

func TestSearchFetchEndToEnd(t *testing.T) {
	client, urls := fakeCatalogClient(t, map[string]platform.RawJSON{
		"/search": fixture(t, "search_cn_koutu.json"),
	})
	spec := New(testContext(t, client))
	result, err := spec.Ops["fetch"].Invoke(context.Background(),
		json.RawMessage(`{"channel":"apple:podcast-search","info":{"q":"凹凸电波","limit":"3","country":"cn"}}`))
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://itunes.apple.com/search?term=%E5%87%B9%E5%87%B8%E7%94%B5%E6%B3%A2&media=podcast&entity=podcast&limit=3&country=cn"
	if !reflect.DeepEqual(*urls, []string{wantURL}) {
		t.Errorf("request urls = %v, want [%s]", *urls, wantURL)
	}
	feed := result.(platform.FetchResult)
	if len(feed.Items) != 1 {
		t.Fatalf("items = %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.SourceID != searchChannelKey || item.Title != "凹凸电波" || item.Summary == nil || *item.Summary != "凹凸电波 · 喜剧 · 395 集" {
		t.Errorf("item = %+v", item)
	}
	var raw map[string]any
	if err := json.Unmarshal(item.Raw, &raw); err != nil {
		t.Fatal(err)
	}
	if _, hasRank := raw["rank"]; hasRank {
		t.Errorf("search raw 不应带 rank: %v", raw)
	}
}

func TestFetchErrors(t *testing.T) {
	ops := New(testContext(t, nil)).Ops
	cases := []struct {
		name   string
		params string
		want   string
	}{
		{"unknown channel", `{"channel":"nope"}`, "未知渠道"},
		{"search missing q", `{"channel":"apple:podcast-search","info":{"q":"  "}}`, "需要 q 参数"},
		{"cursor without pageable", `{"channel":"apple:podcast-chart","cursor":"2"}`, "不支持翻页"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ops["fetch"].Invoke(context.Background(), json.RawMessage(testCase.params))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %v, want contains %q", err, testCase.want)
			}
		})
	}
}
