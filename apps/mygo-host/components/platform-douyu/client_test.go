// 客户端/签名链测试:httptest 模拟斗鱼端点 + 注入 platform.JSRunnerFunc,
// 覆盖签名参数、CDN 回退、逐档拉流、首档失败与 rate=-1 兜底。

package douyu

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"mygo-host/platform"
)

func withDouyuServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldBase, oldMobile := baseURL, mobileBaseURL
	baseURL = srv.URL
	mobileBaseURL = srv.URL
	t.Cleanup(func() {
		baseURL = oldBase
		mobileBaseURL = oldMobile
		srv.Close()
	})
	return srv
}

func testContextWithJS(js platform.JSRunner) *platform.ServiceContext {
	return &platform.ServiceContext{
		HTTP: platform.NewHostClient(),
		JS:   js,
		Now:  func() int64 { return 1700000000000 },
	}
}

type jsCapture struct {
	code string
	fn   string
	args []any
}

func streamsFlowHandler(posts *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/swf_api/homeH5Enc"):
			io.WriteString(w, `{"data":{"room123":"crptext-abc"}}`)
		case strings.HasPrefix(r.URL.Path, "/lapi/live/getH5Play/123"):
			body, _ := io.ReadAll(r.Body)
			*posts = append(*posts, string(body))
			switch {
			case strings.Contains(string(body), "rate=-1"):
				io.WriteString(w, `{"data":{"multirates":[{"rate":2,"name":"原画"},{"rate":4,"name":"蓝光"}],"cdnsWithName":[{"cdn":"scdn-a"},{"cdn":"hw-a"}]}}`)
			case strings.Contains(string(body), "rate=2"):
				io.WriteString(w, `{"data":{"rtmp_url":"https://cdn-hw","rtmp_live":"a.flv?x=1&amp;y=2"}}`)
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}
}

func TestResolveStreamsFlow(t *testing.T) {
	posts := []string{}
	withDouyuServer(t, streamsFlowHandler(&posts))
	capture := &jsCapture{}
	sc := testContextWithJS(platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) {
		capture.code, capture.fn, capture.args = code, fn, args
		return "signed-body", nil
	}))
	streams, err := resolveStreams(context.Background(), sc, "123")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("streams = %d, want 1(第二档 HTTP 500 应跳过)", len(streams))
	}
	assertStreamsFlowResult(t, streams[0])
	assertStreamsFlowJSCall(t, capture)
	assertStreamsFlowPosts(t, posts)
}

func assertStreamsFlowResult(t *testing.T, stream platform.Stream) {
	t.Helper()
	if stream.URL != "https://cdn-hw/a.flv?x=1&y=2" || stream.Format == nil || *stream.Format != "flv" {
		t.Errorf("stream = %+v", stream)
	}
	if stream.Quality == nil || *stream.Quality != "原画" || stream.Rate == nil || *stream.Rate != 2 {
		t.Errorf("quality/rate = %v / %v", stream.Quality, stream.Rate)
	}
	if stream.Headers["referer"] != baseURL+"/123" {
		t.Errorf("headers = %+v", stream.Headers)
	}
}

func assertStreamsFlowJSCall(t *testing.T, capture *jsCapture) {
	t.Helper()
	if capture.fn != "ub98484234" || capture.code != cryptoJS+"\ncrptext-abc" {
		t.Error("js call fn/code 不正确")
	}
	if len(capture.args) != 3 || capture.args[0] != "123" || capture.args[1] != did || capture.args[2] != int64(1700000000) {
		t.Errorf("js args = %+v", capture.args)
	}
}

func assertStreamsFlowPosts(t *testing.T, posts []string) {
	t.Helper()
	if len(posts) != 4 ||
		posts[0] != "signed-body&cdn=&rate=-1" ||
		posts[1] != "signed-body&cdn=hw-a&rate=2" ||
		posts[2] != "signed-body&cdn=hw-a&rate=4" ||
		posts[3] != "signed-body&cdn=scdn-a&rate=4" {
		t.Errorf("posts = %v", posts)
	}
}

func TestFirstQualityFailureAborts(t *testing.T) {
	listCalls := 0
	var mu sync.Mutex
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/swf_api/homeH5Enc"):
			io.WriteString(w, `{"data":{"room7":"crptext"}}`)
		case strings.HasPrefix(r.URL.Path, "/lapi/live/getH5Play/7"):
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "rate=-1") {
				mu.Lock()
				listCalls++
				mu.Unlock()
				io.WriteString(w, `{"data":{"multirates":[{"rate":2,"name":"原画"}],"cdnsWithName":[{"cdn":"a"}]}}`)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})
	sc := testContextWithJS(platform.JSRunnerFunc(func(context.Context, string, string, []any) (any, error) {
		return "signed", nil
	}))
	_, err := resolveStreams(context.Background(), sc, "7")
	if err == nil || !strings.Contains(err.Error(), "H5Play HTTP 500") {
		t.Fatalf("err = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if listCalls != 1 {
		t.Errorf("首档失败后仍继续请求(listCalls=%d)", listCalls)
	}
}

func TestFallbackSingleStream(t *testing.T) {
	fallbackCalls := 0
	var mu sync.Mutex
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/swf_api/homeH5Enc"):
			io.WriteString(w, `{"data":{"room9":"crptext"}}`)
		case strings.HasPrefix(r.URL.Path, "/lapi/live/getH5Play/9"):
			mu.Lock()
			fallbackCalls++
			call := fallbackCalls
			mu.Unlock()
			if call == 1 {
				io.WriteString(w, `{"data":{"cdnsWithName":[{"cdn":"a"}]}}`)
				return
			}
			io.WriteString(w, `{"data":{"rtmp_url":"http://cdn-x","rtmp_live":"live.flv"}}`)
		default:
			http.NotFound(w, r)
		}
	})
	sc := testContextWithJS(platform.JSRunnerFunc(func(context.Context, string, string, []any) (any, error) {
		return "signed", nil
	}))
	streams, err := resolveStreams(context.Background(), sc, "9")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Quality != nil || streams[0].Rate != nil {
		t.Fatalf("fallback streams = %+v", streams)
	}
	if *streams[0].Format != "flv" {
		t.Errorf("format = %v", *streams[0].Format)
	}
}

func TestResolveH5PlayCDNFallback(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "cdn=bad") {
			io.WriteString(w, `{"data":{}}`)
			return
		}
		io.WriteString(w, `{"data":{"rtmp_url":"rtmp://live.example","rtmp_live":"room"}}`)
	})
	sc := testContextWithJS(nil)
	stream, err := resolveH5Play(context.Background(), sc, "5", "signed", 4, []string{"bad", "good"})
	if err != nil {
		t.Fatal(err)
	}
	if stream.URL != "rtmp://live.example/room" || *stream.Format != "rtmp" {
		t.Errorf("stream = %+v", stream)
	}
}

func TestRoomInfoRejectsHTML(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "  <html>blocked</html>")
	})
	if _, err := roomInfo(context.Background(), testContextWithJS(nil), "123"); err == nil ||
		!strings.Contains(err.Error(), "未返回房间信息") {
		t.Fatalf("err = %v", err)
	}
}

func TestRoomInfoDoubleEncoded(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/betard/123" {
			http.NotFound(w, r)
			return
		}
		inner, _ := json.Marshal(map[string]any{"room": map[string]any{
			"room_name": "加密房", "room_id": 9527, "show_status": 1, "videoLoop": 0,
			"owner_name": "主播", "room_pic": "pic", "room_biz_all": map[string]any{"hot": 100},
		}})
		outer, _ := json.Marshal(string(inner))
		w.Write(outer)
	})
	room, err := roomInfo(context.Background(), testContextWithJS(nil), "123")
	if err != nil {
		t.Fatal(err)
	}
	if jsString(room.RoomName) != "加密房" || jsString(room.RoomID) != "9527" {
		t.Errorf("room = %+v", room)
	}
	item := mapRoomItem("123", room, testNow)
	if *item.LiveStatus != platform.LiveStatusLive || item.Online == nil || *item.Online != 100 {
		t.Errorf("item = %+v", item)
	}
}

func TestSignUnavailableWithoutJSRunner(t *testing.T) {
	_, err := sign(context.Background(), &platform.ServiceContext{}, "crptext", "1")
	if !errors.Is(err, platform.ErrJSRunnerUnavailable) {
		t.Fatalf("err = %v", err)
	}
}
