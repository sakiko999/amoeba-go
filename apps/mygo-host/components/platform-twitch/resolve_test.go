// 直播播放解析测试(本地 httptest:persisted token 查询 + usher master)。

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

const goodTokenValue = `{"geoblock_reason":"","blackout_enabled":false}`

func startPlayServer(t *testing.T) *platform.ServiceContext {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/gql", func(w http.ResponseWriter, r *http.Request) { handleTokenQuery(t, w, r) })
	mux.HandleFunc("/hls/", func(w http.ResponseWriter, r *http.Request) { handleMaster(t, w, r) })
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	oldGQL, oldUsher := gqlURL, usherBase
	gqlURL = server.URL + "/gql"
	usherBase = server.URL + "/hls"
	t.Cleanup(func() { gqlURL, usherBase = oldGQL, oldUsher })
	return &platform.ServiceContext{
		HTTP: &platform.HostClient{HTTP: server.Client()},
		Now:  func() int64 { return fixedAt },
		Logf: func(string, ...any) {},
	}
}

func handleTokenQuery(t *testing.T, w http.ResponseWriter, r *http.Request) {
	var body struct {
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
		Extensions    map[string]any `json:"extensions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.OperationName != "PlaybackAccessToken" {
		t.Errorf("operationName = %q", body.OperationName)
	}
	persisted, _ := body.Extensions["persistedQuery"].(map[string]any)
	if persisted["sha256Hash"] != playbackTokenHash {
		t.Errorf("persistedQuery = %v", body.Extensions)
	}
	login, _ := body.Variables["login"].(string)
	token := func(value string) any {
		return map[string]any{"data": map[string]any{
			"streamPlaybackAccessToken": map[string]any{"signature": "sig-" + login, "value": value},
		}}
	}
	w.Header().Set("Content-Type", "application/json")
	switch login {
	case "notoken":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"streamPlaybackAccessToken": nil}})
	case "geo":
		_ = json.NewEncoder(w).Encode(token(`{"geoblock_reason":"US","blackout_enabled":false}`))
	case "blackout":
		_ = json.NewEncoder(w).Encode(token(`{"geoblock_reason":"","blackout_enabled":true}`))
	default:
		_ = json.NewEncoder(w).Encode(token(goodTokenValue))
	}
}

func handleMaster(t *testing.T, w http.ResponseWriter, r *http.Request) {
	login := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/hls/"), ".m3u8")
	if login == "shroud" {
		query := r.URL.Query()
		if query.Get("sig") != "sig-shroud" || query.Get("token") != goodTokenValue {
			t.Errorf("usher query sig/token = %q/%q", query.Get("sig"), query.Get("token"))
		}
		if query.Get("platform") != "web" || query.Get("supported_codecs") != "h264" || query.Get("p") == "" {
			t.Errorf("usher query = %v", query)
		}
	}
	switch login {
	case "shroud":
		_, _ = w.Write([]byte(masterFixture))
	case "nvt":
		_, _ = w.Write([]byte("#EXTM3U\n"))
	case "offline":
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func callPlay(t *testing.T, spec platform.Spec, params string) (any, error) {
	t.Helper()
	return spec.Ops["play"].Invoke(context.Background(), json.RawMessage(params))
}

func TestPlayResolveLiveByID(t *testing.T) {
	spec := New(startPlayServer(t))
	result, err := callPlay(t, spec, `{"id":"shroud"}`)
	if err != nil {
		t.Fatal(err)
	}
	play := result.(platform.PlayOpResult)
	checkStr(t, "play.id", play.ID, "shroud")
	if len(play.Streams) != 4 {
		t.Fatalf("streams = %d, want 4", len(play.Streams))
	}
	checkIntPtr(t, "streams[0].rate", play.Streams[0].Rate, 1080)
	checkStrPtr(t, "streams[0].format", play.Streams[0].Format, "hls")
	checkStrPtr(t, "streams[0].quality", play.Streams[0].Quality, "1080p60 (source)")
}

func TestPlayResolveLiveByURL(t *testing.T) {
	spec := New(startPlayServer(t))
	result, err := callPlay(t, spec, `{"url":"https://www.twitch.tv/shroud"}`)
	if err != nil {
		t.Fatal(err)
	}
	play := result.(platform.PlayOpResult)
	if play.ID != "shroud" || len(play.Streams) != 4 {
		t.Errorf("play = %+v", play)
	}
}

func TestPlayErrors(t *testing.T) {
	spec := New(startPlayServer(t))
	cases := []struct {
		name     string
		params   string
		contains string
	}{
		{"无效频道名", `{"id":"bad!"}`, "无效频道名"},
		{"无法路由", `{"url":"https://example.com/x"}`, "无法从 URL 解析播放路由"},
		{"无 token", `{"id":"notoken"}`, "无法获取播放 token"},
		{"地区受限", `{"id":"geo"}`, "地区/版权受限(US)"},
		{"blackout", `{"id":"blackout"}`, "版权限制(blackout)"},
		{"master 无档位", `{"id":"nvt"}`, "master 播放列表无可用档位"},
		{"master 500", `{"id":"boom"}`, "HTTP 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := callPlay(t, spec, tc.params); err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("err = %v, want 包含 %q", err, tc.contains)
			}
		})
	}

	_, err := callPlay(t, spec, `{"id":"offline"}`)
	var notFound *platform.NotFoundError
	if !errors.As(err, &notFound) {
		t.Errorf("master 404 err = %v, want NotFoundError", err)
	}
}
