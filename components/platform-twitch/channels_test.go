// 渠道 fetch/翻页协议测试(本地 httptest 假 GraphQL 端点,不依赖真实网络)。

package twitch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mygo-host/platform"
)

// startGQLServer 起假 GraphQL 端点并注入 ServiceContext;calls 返回(查询串, client-id)。
func startGQLServer(t *testing.T, respond func(query string) any) (*platform.ServiceContext, func() ([]string, []string)) {
	t.Helper()
	var mu sync.Mutex
	var queries, clientIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		queries = append(queries, body.Query)
		clientIDs = append(clientIDs, r.Header.Get("client-id"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": respond(body.Query)})
	}))
	t.Cleanup(server.Close)
	old := gqlURL
	gqlURL = server.URL
	t.Cleanup(func() { gqlURL = old })
	sc := &platform.ServiceContext{
		HTTP: &platform.HostClient{HTTP: server.Client()},
		Now:  func() int64 { return fixedAt },
		Logf: func(string, ...any) {},
	}
	calls := func() ([]string, []string) {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), queries...), append([]string(nil), clientIDs...)
	}
	return sc, calls
}

func callFetch(t *testing.T, spec platform.Spec, params string) (any, error) {
	t.Helper()
	return spec.Ops["fetch"].Invoke(context.Background(), json.RawMessage(params))
}

func streamNodeMap(login, title string) map[string]any {
	return map[string]any{
		"id":              "1",
		"title":           title,
		"viewersCount":    100,
		"createdAt":       "2026-10-20T08:00:00.000Z",
		"broadcaster":     map[string]any{"login": login, "displayName": strings.ToUpper(login)},
		"game":            map[string]any{"name": "Game", "slug": "game"},
		"previewImageURL": "https://img.example/" + login + ".jpg",
	}
}

func streamPageData(nodes []any, hasMore bool, endCursor string) map[string]any {
	edges := make([]any, 0, len(nodes))
	for _, node := range nodes {
		edges = append(edges, map[string]any{"cursor": endCursor, "node": node})
	}
	return map[string]any{"streams": map[string]any{
		"edges":    edges,
		"pageInfo": map[string]any{"hasNextPage": hasMore, "endCursor": endCursor},
	}}
}

func TestFetchLiveChannel(t *testing.T) {
	sc, calls := startGQLServer(t, func(string) any {
		return map[string]any{
			"u0": map[string]any{
				"login": "shroud", "displayName": "Shroud", "description": "FPS streamer",
				"stream": map[string]any{
					"id": "1", "title": "Ranked grind", "viewersCount": 12345,
					"createdAt": "2026-10-20T08:00:00.000Z", "type": "live",
					"game": map[string]any{"name": "VALORANT"},
				},
			},
			"u1": nil,
			"u2": map[string]any{"login": "xqc", "displayName": "xQc", "description": "", "stream": nil},
		}
	})
	spec := New(sc)
	result, err := callFetch(t, spec,
		`{"channel":"twitch:live","info":{"roomIds":"Shroud, https://www.twitch.tv/riotgames, xqc!, @xqc"}}`)
	if err != nil {
		t.Fatal(err)
	}
	items := result.(platform.FetchResult).Items
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	checkStr(t, "items[0].id", items[0].ID, "twitch:shroud")
	checkStr(t, "items[0].title", items[0].Title, "Ranked grind")
	checkIntPtr(t, "items[0].online", items[0].Online, 12345)
	if items[1].LiveStatus == nil || *items[1].LiveStatus != platform.LiveStatusOffline {
		t.Errorf("items[1].liveStatus = %v", items[1].LiveStatus)
	}
	checkStr(t, "items[1].title", items[1].Title, "xQc（离线）")

	queries, clientIDs := calls()
	if len(queries) != 1 {
		t.Fatalf("gql 请求数 = %d, want 1", len(queries))
	}
	for _, fragment := range []string{
		`u0: user(login: "shroud")`, `u1: user(login: "riotgames")`, `u2: user(login: "xqc")`,
	} {
		if !strings.Contains(queries[0], fragment) {
			t.Errorf("query 缺少 %s: %s", fragment, queries[0])
		}
	}
	if clientIDs[0] != webClientID {
		t.Errorf("client-id = %q, want 网页 Client-ID", clientIDs[0])
	}
}

func TestFetchLiveChannelErrors(t *testing.T) {
	sc := &platform.ServiceContext{HTTP: platform.NewHostClient(), Now: func() int64 { return fixedAt }}
	spec := New(sc)
	if _, err := callFetch(t, spec, `{"channel":"twitch:live","info":{}}`); err == nil ||
		!strings.Contains(err.Error(), "需要 roomIds") {
		t.Errorf("空 roomIds err = %v", err)
	}
	if _, err := callFetch(t, spec, `{"channel":"twitch:live","info":{"roomIds":"bad!"}}`); err == nil ||
		!strings.Contains(err.Error(), "没有合法频道名") {
		t.Errorf("非法 roomIds err = %v", err)
	}

	allNull, _ := startGQLServer(t, func(string) any { return map[string]any{"u0": nil} })
	_, err := callFetch(t, New(allNull), `{"channel":"twitch:live","info":{"roomIds":"shroud"}}`)
	if err == nil || !strings.Contains(err.Error(), "全部频道拉取失败") {
		t.Errorf("全部不存在 err = %v", err)
	}
}

func TestFetchHotChannelPagination(t *testing.T) {
	sc, calls := startGQLServer(t, func(query string) any {
		switch {
		case strings.Contains(query, `after: "c1"`):
			return streamPageData([]any{streamNodeMap("hot2", "B")}, true, "c2")
		case strings.Contains(query, `after: "c2"`):
			return streamPageData([]any{streamNodeMap("hot3", "C")}, false, "")
		default:
			return streamPageData([]any{streamNodeMap("hot1", "A")}, true, "c1")
		}
	})
	spec := New(sc)

	result, err := callFetch(t, spec, `{"channel":"twitch:hot"}`)
	if err != nil {
		t.Fatal(err)
	}
	first := result.(platform.FetchResult).Items
	if len(first) != 1 || first[0].ID != "twitch:hot1" {
		t.Fatalf("首页 items = %+v", first)
	}

	result, err = callFetch(t, spec, `{"channel":"twitch:hot","cursor":""}`)
	if err != nil {
		t.Fatal(err)
	}
	page := result.(platform.FetchPage)
	if len(page.Items) != 1 || page.Items[0].ID != "twitch:hot2" {
		t.Fatalf("首次翻页 items = %+v", page.Items)
	}
	if page.Cursor == nil || *page.Cursor != "c2" {
		t.Errorf("首次翻页 cursor = %v", page.Cursor)
	}

	result, err = callFetch(t, spec, `{"channel":"twitch:hot","cursor":"c2"}`)
	if err != nil {
		t.Fatal(err)
	}
	last := result.(platform.FetchPage)
	if len(last.Items) != 1 || last.Items[0].ID != "twitch:hot3" || last.Cursor != nil {
		t.Errorf("末页 = %+v", last)
	}

	queries, clientIDs := calls()
	if len(queries) != 4 {
		t.Fatalf("gql 请求数 = %d, want 4", len(queries))
	}
	for _, id := range clientIDs {
		if id != mobileClientID {
			t.Errorf("client-id = %q, want 移动端 Client-ID", id)
		}
	}
	if !strings.Contains(queries[2], `after: "c1"`) {
		t.Errorf("首次翻页第二次请求缺少 after c1: %s", queries[2])
	}
}

func TestFetchCategoriesChannel(t *testing.T) {
	sc, calls := startGQLServer(t, func(query string) any {
		if !strings.Contains(query, "games(first: 30)") {
			t.Errorf("query = %s", query)
		}
		return map[string]any{"games": map[string]any{"edges": []any{
			map[string]any{"node": map[string]any{
				"id": "509658", "name": "Just Chatting", "slug": "just-chatting",
				"viewersCount": 1234, "boxArtURL": "https://box.example/jc.jpg",
			}},
			map[string]any{"node": map[string]any{"name": "No Slug", "slug": ""}},
			map[string]any{"node": nil},
		}}}
	})
	spec := New(sc)
	result, err := callFetch(t, spec, `{"channel":"twitch:categories"}`)
	if err != nil {
		t.Fatal(err)
	}
	items := result.(platform.FetchResult).Items
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	checkStr(t, "category id", item.ID, "twitch-category:just-chatting")
	if item.Kind != platform.KindCategory {
		t.Errorf("kind = %s", item.Kind)
	}
	checkStrPtr(t, "category summary", item.Summary, "1234 人在看")
	checkStrPtr(t, "category thumbnail", item.Thumbnail, "https://box.example/jc.jpg")
	checkStr(t, "categoryInfo", item.CategoryInfo["category"], "just-chatting")

	if _, err := callFetch(t, spec, `{"channel":"twitch:categories","cursor":"1"}`); err == nil ||
		!strings.Contains(err.Error(), "不支持翻页") {
		t.Errorf("单页渠道翻页 err = %v", err)
	}
	queries, _ := calls()
	if len(queries) != 1 {
		t.Errorf("gql 请求数 = %d, want 1", len(queries))
	}
}

func TestFetchUserSearchChannel(t *testing.T) {
	sc, calls := startGQLServer(t, func(query string) any {
		if !strings.Contains(query, "searchFor") {
			t.Errorf("query = %s", query)
		}
		return map[string]any{"searchFor": map[string]any{"channels": map[string]any{"items": []any{
			nil,
			map[string]any{
				"login": "Shroud", "displayName": "", "description": " FPS streamer ",
				"profileImageURL": "https://img.example/shroud-70.png",
				"followers":       map[string]any{"totalCount": 12345},
			},
		}}}}
	})
	spec := New(sc)
	result, err := callFetch(t, spec, `{"channel":"twitch:user:search","info":{"keyword":" shroud "}}`)
	if err != nil {
		t.Fatal(err)
	}
	items := result.(platform.FetchResult).Items
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	checkStr(t, "search id", item.ID, "tw-shroud")
	checkStr(t, "search title", item.Title, "shroud")
	checkStrPtr(t, "search summary", item.Summary, "1.2万 关注者 · FPS streamer")
	if item.Kind != platform.KindLive {
		t.Errorf("kind = %s", item.Kind)
	}
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusOffline {
		t.Errorf("liveStatus = %v", item.LiveStatus)
	}
	checkStrPtr(t, "search thumbnail(离线回退头像)", item.Thumbnail, "https://img.example/shroud-70.png")
	if item.Author == nil {
		t.Fatal("author 缺失")
	}
	checkStrPtr(t, "search avatar", item.Author.Avatar, "https://img.example/shroud-70.png")

	if _, err := callFetch(t, spec, `{"channel":"twitch:user:search","info":{}}`); err == nil ||
		!strings.Contains(err.Error(), "需要 keyword 参数") {
		t.Errorf("空 keyword err = %v", err)
	}
	queries, _ := calls()
	if len(queries) != 1 {
		t.Errorf("gql 请求数 = %d, want 1", len(queries))
	}
}

func TestSearchLiveOp(t *testing.T) {
	sc, calls := startGQLServer(t, func(query string) any {
		if strings.Contains(query, "searchFor") {
			return map[string]any{"searchFor": map[string]any{"channels": map[string]any{"items": []any{
				map[string]any{"login": "Shroud", "displayName": "Shroud"},
				map[string]any{"login": "xqc", "displayName": "xQc"},
			}}}}
		}
		return map[string]any{
			"u0": map[string]any{
				"login": "shroud", "displayName": "Shroud",
				"stream": map[string]any{
					"id": "s1", "title": "Ranked grind", "viewersCount": 12345,
					"createdAt": "2026-10-20T08:00:00.000Z", "type": "live",
					"game": map[string]any{"name": "VALORANT"},
				},
			},
			"u1": map[string]any{"login": "xqc", "displayName": "xQc", "stream": nil},
		}
	})
	spec := New(sc)
	result, err := spec.Ops["search"].Invoke(context.Background(),
		json.RawMessage(`{"keyword":"shroud","scope":"live"}`))
	if err != nil {
		t.Fatal(err)
	}
	items := result.(platform.FetchPage).Items
	if len(items) != 1 {
		t.Fatalf("items = %+v, want 只有在播的 shroud", items)
	}
	item := items[0]
	checkStr(t, "live id", item.ID, "twitch:shroud")
	checkStr(t, "live sourceID", item.SourceID, "twitch:live:search")
	checkStr(t, "live title", item.Title, "Ranked grind")
	checkIntPtr(t, "live online", item.Online, 12345)
	if item.LiveStatus == nil || *item.LiveStatus != platform.LiveStatusLive {
		t.Errorf("live liveStatus = %v", item.LiveStatus)
	}
	if item.Author == nil || item.Author.Name != "Shroud" {
		t.Errorf("live author = %+v", item.Author)
	}

	queries, _ := calls()
	if len(queries) != 2 {
		t.Fatalf("gql 请求数 = %d, want 2(搜索 + enrich)", len(queries))
	}
	if !strings.Contains(queries[1], `u0: user(login: "shroud")`) {
		t.Errorf("enrich 查询应按别名批量查 user: %s", queries[1])
	}
}
