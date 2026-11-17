// twitch:vod 渠道、回放播放解析与 VOD 弹幕测试(本地 httptest:
// videos 列表查询、VOD token 查询、usher VOD master、评论分页)。

package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mygo-host/platform"
)

const (
	vodGoodID    = "2929172303"
	vodMasterSig = "sig-vod-" + vodGoodID
)

func startVODServer(t *testing.T) *platform.ServiceContext {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/gql", handleVODGQL)
	mux.HandleFunc("/vod/", handleVODMaster(t))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	oldGQL, oldUsher := gqlURL, vodUsherBase
	gqlURL = server.URL + "/gql"
	vodUsherBase = server.URL + "/vod"
	t.Cleanup(func() { gqlURL, vodUsherBase = oldGQL, oldUsher })
	return &platform.ServiceContext{
		HTTP: &platform.HostClient{HTTP: server.Client()},
		Now:  func() int64 { return fixedAt },
		Logf: func(string, ...any) {},
	}
}

func handleVODGQL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
		Extensions    map[string]any `json:"extensions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	w.Header().Set("Content-Type", "application/json")
	switch body.OperationName {
	case "PlaybackAccessToken":
		writeVODTokenQuery(w, body.Variables)
	case "VideoCommentsByOffsetOrCursor":
		writeCommentsPage(w, body.Extensions, body.Variables)
	default:
		if strings.Contains(body.Query, "videos(first") {
			writeVideosPage(w, body.Query)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}
}

func writeVODTokenQuery(w http.ResponseWriter, variables map[string]any) {
	vodID, _ := variables["vodID"].(string)
	if vodID == "99900001" || variables["isVod"] != true {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"videoPlaybackAccessToken": nil}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		"videoPlaybackAccessToken": map[string]any{
			"signature": vodMasterSig,
			"value":     goodTokenValue,
		},
	}})
}

func writeVideosPage(w http.ResponseWriter, query string) {
	login := "unknown"
	if match := strings.Index(query, `user(login: "shroud")`); match >= 0 {
		login = "shroud"
	}
	if login != "shroud" {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"user": nil}})
		return
	}
	after := ""
	if strings.Contains(query, `after: "c1"`) {
		after = "c1"
	} else if strings.Contains(query, `after: "c2"`) {
		after = "c2"
	}
	edges := `[{"cursor":"c1","node":{"id":"111","title":"第一场","lengthSeconds":3600,
		"createdAt":"2026-01-02T03:04:05Z","viewCount":1234,"status":"RECORDED",
		"previewThumbnailURL":"https://example.com/p1.jpg",
		"owner":{"login":"shroud","displayName":"shroud"}}},
		{"cursor":"c1","node":{"id":"222","title":"录制中的场次","lengthSeconds":100,
		"createdAt":"2026-01-03T03:04:05Z","viewCount":5,"status":"RECORDING",
		"owner":{"login":"shroud","displayName":""}}}]`
	switch after {
	case "c1":
		edges = `[{"cursor":"c2","node":{"id":"333","title":"第二页","lengthSeconds":60,
			"createdAt":"2026-01-04T03:04:05Z","status":"RECORDED"}}]`
		_ = json.NewEncoder(w).Encode(videosEnvelope(edges, true, 1607))
		return
	case "c2":
		_ = json.NewEncoder(w).Encode(videosEnvelope(`[]`, false, 1607))
		return
	}
	_ = json.NewEncoder(w).Encode(videosEnvelope(edges, true, 1607))
}

func videosEnvelope(edges string, hasNext bool, total int) map[string]any {
	return map[string]any{"data": map[string]any{"user": map[string]any{"videos": map[string]any{
		"totalCount": total,
		"edges":      json.RawMessage(edges),
		"pageInfo":   map[string]any{"hasNextPage": hasNext},
	}}}}
}

func handleVODMaster(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/vod/"), ".m3u8")
		if id == vodGoodID {
			query := r.URL.Query()
			if query.Get("sig") != vodMasterSig || query.Get("token") != goodTokenValue {
				t.Errorf("vod usher query sig/token = %q/%q", query.Get("sig"), query.Get("token"))
			}
			_, _ = w.Write([]byte(masterFixture))
			return
		}
		switch id {
		case "10000001":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("vod_manifest_restricted"))
		case "20000002":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func fetchVOD(t *testing.T, sc *platform.ServiceContext) platform.DataSource {
	t.Helper()
	channel := &vodChannel{sc: sc}
	return channel.GetSource(platform.SourceInfo{"login": "twitch.tv/shroud"})
}

func TestVODChannelFetch(t *testing.T) {
	sc := startVODServer(t)
	result, err := fetchVOD(t, sc).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	first := result.Items[0]
	checkStr(t, "id", first.ID, "twitch:vod:111")
	checkStr(t, "sourceId", first.SourceID, "twitch:vod")
	checkStr(t, "kind", string(first.Kind), "video")
	checkStr(t, "title", first.Title, "第一场")
	checkStrPtr(t, "url", first.URL, "https://www.twitch.tv/videos/111")
	checkStrPtr(t, "thumbnail", first.Thumbnail, "https://example.com/p1.jpg")
	checkStrPtr(t, "summary", first.Summary, "1234 次观看")
	if first.Author == nil || first.Author.Name != "shroud" {
		t.Errorf("author = %+v", first.Author)
	}
	if first.Duration == nil || *first.Duration != 3600 {
		t.Errorf("duration = %v", first.Duration)
	}
	if first.PublishedAt == nil {
		t.Errorf("publishedAt 缺失")
	}
	recording := result.Items[1]
	checkStrPtr(t, "recording.summary", recording.Summary, "录制中 · 5 次观看")
	if recording.Author != nil {
		t.Errorf("空 displayName 不应产出 author")
	}
	if result.Total == nil || *result.Total != 1607 {
		t.Errorf("total = %v, want 1607", result.Total)
	}
}

func TestVODChannelFetchMore(t *testing.T) {
	sc := startVODServer(t)
	source := fetchVOD(t, sc)
	pageable := source.(platform.PageableSource)
	// 首次翻页无游标:先补首页拿 endCursor 再取下一页;末页(hasNext=false)不带 cursor。
	page, err := pageable.FetchMore(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "twitch:vod:333" {
		t.Fatalf("page1 items = %+v", page.Items)
	}
	if page.Cursor == nil || *page.Cursor != "c2" {
		t.Fatalf("cursor = %v, want c2", page.Cursor)
	}
	// 带游标翻到末页:空条目、无 cursor。
	page2, err := pageable.FetchMore(context.Background(), "c2", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 0 || page2.Cursor != nil {
		t.Fatalf("page2 = %+v cursor=%v", page2.Items, page2.Cursor)
	}
}

func TestVODChannelInvalidLogin(t *testing.T) {
	sc := startVODServer(t)
	if _, err := (&vodChannel{sc: sc}).GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "需要 login") {
		t.Errorf("空 login err = %v", err)
	}
	if _, err := (&vodChannel{sc: sc}).GetSource(platform.SourceInfo{"login": "bad!"}).Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "无效频道名") {
		t.Errorf("bad login err = %v", err)
	}
}

func TestPlayResolveVOD(t *testing.T) {
	spec := New(startVODServer(t))
	// id-only 默认 kind=video(有 video resolver)。
	result, err := callPlay(t, spec, `{"id":"`+vodGoodID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	play := result.(platform.PlayOpResult)
	checkStr(t, "play.id", play.ID, vodGoodID)
	if len(play.Streams) != 4 {
		t.Fatalf("streams = %d, want 4", len(play.Streams))
	}
	checkIntPtr(t, "streams[0].rate", play.Streams[0].Rate, 1080)

	byURL, err := callPlay(t, spec, `{"url":"https://www.twitch.tv/videos/`+vodGoodID+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if urlPlay := byURL.(platform.PlayOpResult); urlPlay.ID != vodGoodID || len(urlPlay.Streams) != 4 {
		t.Errorf("url play = %+v", urlPlay)
	}
}

func TestPlayVODErrors(t *testing.T) {
	spec := New(startVODServer(t))
	cases := []struct {
		name     string
		params   string
		contains string
	}{
		{"无效 ID", `{"id":"shroud"}`, "无效回放 ID"},
		{"token null", `{"id":"99900001"}`, "不存在、已删除或已过期"},
		{"sub-only", `{"id":"10000001"}`, "需要登录或订阅"},
		{"master 404", `{"id":"20000002"}`, "不存在或已删除"},
		{"master 500", `{"id":"30000003"}`, "HTTP 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := callPlay(t, spec, tc.params); err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("err = %v, want 包含 %q", err, tc.contains)
			}
		})
	}
	_, err := callPlay(t, spec, `{"id":"20000002"}`)
	var notFound *platform.NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("usher 404 err = %v, want NotFoundError", err)
	}
}
