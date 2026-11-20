// enter/签名/ttwid 测试:httptest 假上游 + fake JSRunner,断言参数构造、
// cookie/UA 头与错误文案;不访问真实外网。

package douyin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestEnterRoomParams(t *testing.T) {
	want := "aid=6383&app_name=douyin_web&live_id=1&device_platform=web&language=zh-CN&browser_language=zh-CN&browser_platform=Win32&browser_name=Chrome&browser_version=125.0.0.0&web_rid=6753235366&msToken="
	got := enterRoomParams("6753235366")
	if got != want {
		t.Errorf("enterRoomParams = %q", got)
	}
	if len(got) != 193 {
		t.Errorf("len = %d, want 193", len(got))
	}
}

func TestSignDouyinURL(t *testing.T) {
	capture := &jsCall{}
	sc := testContext()
	sc.JS = captureJS(capture)
	raw := "https://live.douyin.com/webcast/room/web/enter/?aid=6383&msToken="
	out, err := signDouyinURL(context.Background(), sc, raw, uaEnter)
	if err != nil {
		t.Fatal(err)
	}
	assertSignCall(t, capture)
	assertSignedURL(t, raw, out)
}

func assertSignCall(t *testing.T, capture *jsCall) {
	t.Helper()
	if capture.fn != "getABogus" || capture.code != abogusJS {
		t.Errorf("js fn/code 不正确(fn=%q)", capture.fn)
	}
	if len(capture.args) != 2 || capture.args[1] != uaEnter {
		t.Fatalf("js args = %+v", capture.args)
	}
}

func assertSignedURL(t *testing.T, raw, out string) {
	t.Helper()
	fields := strings.Split(strings.TrimPrefix(out, raw), "&")
	if len(fields) != 3 || fields[0] != "" {
		t.Fatalf("signed = %q", out)
	}
	token, ok := strings.CutPrefix(fields[1], "msToken=")
	if !ok || len(token) != 107 {
		t.Errorf("msToken = %q", fields[1])
	}
	if fields[2] != "a_bogus=ab%2Fc%3D" {
		t.Errorf("a_bogus = %q", fields[2])
	}
}

func TestSignDouyinURLWithoutJS(t *testing.T) {
	sc := testContext()
	sc.JS = nil
	_, err := signDouyinURL(context.Background(), sc, "https://x.test/?a=1", uaEnter)
	if !errors.Is(err, platform.ErrJSRunnerUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestWarmupCookieParse(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	calls := 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Add("Set-Cookie", "ttwid=abc; Path=/; HttpOnly")
		w.Header().Add("Set-Cookie", "sid=1; Path=/")
		w.Header().Add("Set-Cookie", "broken")
	})
	sc := testContext()
	if got := douyinCookie(context.Background(), sc); got != "ttwid=abc; sid=1" {
		t.Fatalf("cookie = %q", got)
	}
	if again := douyinCookie(context.Background(), sc); again != "ttwid=abc; sid=1" {
		t.Errorf("第二次 cookie = %q", again)
	}
	if calls != 1 {
		t.Errorf("warmup 请求次数 = %d, want 1", calls)
	}
}

func TestWarmupFallbackDefault(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if got := douyinCookie(context.Background(), testContext()); got != defaultTTWID {
		t.Fatalf("cookie = %q", got)
	}
}

func TestCredentialsCookiePriority(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	creds := platform.NewCredentials(filepath.Join(t.TempDir(), "credentials.json"))
	if err := creds.Set("douyin", "sessionid=login"); err != nil {
		t.Fatal(err)
	}
	sc := testContext()
	sc.Creds = creds
	if got := douyinCookie(context.Background(), sc); got != "sessionid=login" {
		t.Fatalf("cookie = %q", got)
	}
}

func withLiveServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldLive, oldReflow := liveBase, reflowBase
	liveBase, reflowBase = srv.URL, srv.URL
	t.Cleanup(func() {
		liveBase, reflowBase = oldLive, oldReflow
		srv.Close()
	})
}

func TestResolveLiveStreamsViaReflow(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	enterBody := fixture(t, "enter_room.json")
	reflowBody := fixture(t, "reflow_room.json")
	var enterQuery url.Values
	var enterRaw string
	var enterHeader http.Header
	var reflowQuery url.Values
	var reflowHeader http.Header
	enterCalls, reflowCalls := 0, 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "ttwid=warm; Path=/")
		case "/webcast/room/web/enter/":
			enterCalls++
			enterQuery, enterRaw, enterHeader = r.URL.Query(), r.URL.RawQuery, r.Header.Clone()
			w.Write(enterBody)
		case "/webcast/room/reflow/info/":
			reflowCalls++
			reflowQuery, reflowHeader = r.URL.Query(), r.Header.Clone()
			w.Write(reflowBody)
		default:
			http.NotFound(w, r)
		}
	})
	capture := &jsCall{}
	sc := testContext()
	sc.JS = captureJS(capture)
	streams, err := resolveLiveStreams(context.Background(), sc, "6753235366")
	if err != nil {
		t.Fatal(err)
	}
	assertEnterRequest(t, enterQuery, enterRaw, enterHeader, capture)
	if enterCalls != 1 || reflowCalls != 1 {
		t.Fatalf("enter/reflow 次数 = %d/%d, want 1/1(enter 成功响应必须复用)", enterCalls, reflowCalls)
	}
	assertReflowRequest(t, reflowQuery, reflowHeader)
	assertReflowStreams(t, streams)
}

func assertReflowRequest(t *testing.T, query url.Values, header http.Header) {
	t.Helper()
	want := "app_id=6383&live_id=1&room_id=7300000000000000001&sec_user_id=&type_id=0&version_code=99.99.99"
	if got := query.Encode(); got != want {
		t.Errorf("reflow query = %q, want %q", got, want)
	}
	if query.Get("msToken") != "" || query.Get("a_bogus") != "" {
		t.Errorf("reflow 不应签名: %v", query)
	}
	if header.Get("user-agent") != uaEnter || header.Get("referer") != liveOrigin ||
		header.Get("authority") != "live.douyin.com" || header.Get("cookie") != "ttwid=warm" {
		t.Errorf("reflow headers = %v", header)
	}
}

func TestResolveLiveStreamsEnterStreamFirst(t *testing.T) {
	enterStream := fixture(t, "stream_sdk.json")
	reflowCalls := 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "ttwid=warm; Path=/")
		case "/webcast/room/web/enter/":
			w.Write([]byte(`{"status_code":0,"data":{"data":[{"id_str":"1","stream_url":` +
				string(enterStream) + `}]}}`))
		case "/webcast/room/reflow/info/":
			reflowCalls++
		default:
			http.NotFound(w, r)
		}
	})
	streams, err := resolveLiveStreams(context.Background(), testContext(), "6753235366")
	if err != nil {
		t.Fatal(err)
	}
	if reflowCalls != 0 {
		t.Errorf("enter 已带流仍请求了 reflow(%d 次)", reflowCalls)
	}
	if len(streams) != 2 || streams[0].URL != "https://pull-flv.douyincdn.com/stage/stream-3.flv" {
		t.Errorf("streams = %+v", streams)
	}
}

func assertEnterRequest(t *testing.T, query url.Values, rawQuery string, header http.Header, capture *jsCall) {
	t.Helper()
	if query.Get("aid") != "6383" || query.Get("web_rid") != "6753235366" || query.Get("browser_name") != "Chrome" {
		t.Errorf("query = %v", query)
	}
	tokens := query["msToken"]
	if len(tokens) != 2 || tokens[0] != "" || len(tokens[1]) != 107 {
		t.Errorf("msToken = %v", tokens)
	}
	if query.Get("a_bogus") != "ab/c=" {
		t.Errorf("a_bogus = %q", query.Get("a_bogus"))
	}
	queryArg, _ := capture.args[0].(string)
	if capture.fn != "getABogus" || rawQuery != queryArg+"&a_bogus=ab%2Fc%3D" {
		t.Errorf("签名 query 与请求不一致(capture=%+v)", capture)
	}
	if header.Get("user-agent") != uaEnter || header.Get("cookie") != "ttwid=warm" {
		t.Errorf("headers = %v", header)
	}
	if header.Get("referer") != liveOrigin+"/6753235366" || header.Get("authority") != "live.douyin.com" {
		t.Errorf("headers = %v", header)
	}
}

func assertReflowStreams(t *testing.T, streams []platform.Stream) {
	t.Helper()
	want := []struct {
		url     string
		quality string
		rate    int
		format  string
	}{
		{"https://pull-flv.douyincdn.com/stage/stream-5.flv?k=1&t=2", "蓝光", 5, "flv"},
		{"https://pull-flv.douyincdn.com/stage/stream-4.flv", "高清", 4, "flv"},
		{"https://pull-flv.douyincdn.com/stage/stream-3.flv", "标清", 3, "flv"},
		{"https://pull-hls.douyincdn.com/stage/stream-2.m3u8", "流畅", 2, "hls"},
		{"http://pull-origin.douyincdn.com/stage/stream-1.flv", "原画", 1, "flv"},
	}
	if len(streams) != len(want) {
		t.Fatalf("streams = %d, want %d: %+v", len(streams), len(want), streams)
	}
	for i, w := range want {
		got := streams[i]
		if got.URL != w.url || got.Format == nil || *got.Format != w.format ||
			got.Quality == nil || *got.Quality != w.quality || got.Rate == nil || *got.Rate != w.rate {
			t.Errorf("stream[%d] = %+v, want %+v", i, got, w)
		}
		if got.Headers["authority"] != "live.douyin.com" || got.Headers["referer"] != liveOrigin ||
			got.Headers["user-agent"] != uaEnter {
			t.Errorf("stream[%d].headers = %+v", i, got.Headers)
		}
	}
}

func TestResolveLiveStreamsHTMLFallback(t *testing.T) {
	enterBody := fixture(t, "enter_room.json")
	html := fixture(t, "live_page.html")
	reflowCalls, htmlCalls := 0, 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "ttwid=warm; Path=/")
		case "/webcast/room/web/enter/":
			w.Write(enterBody)
		case "/webcast/room/reflow/info/":
			reflowCalls++
			w.Write([]byte(`{"data":{}}`))
		case "/6753235366":
			htmlCalls++
			w.Write(html)
		default:
			http.NotFound(w, r)
		}
	})
	streams, err := resolveLiveStreams(context.Background(), testContext(), "6753235366")
	if err != nil {
		t.Fatal(err)
	}
	if reflowCalls != 1 || htmlCalls != 1 {
		t.Fatalf("reflow/html 次数 = %d/%d, want 1/1", reflowCalls, htmlCalls)
	}
	if len(streams) != 3 {
		t.Fatalf("streams = %+v", streams)
	}
	if streams[0].URL != "https://pull-flv.douyincdn.com/stage/html-5.flv?a=1&b=2" ||
		streams[0].Quality == nil || *streams[0].Quality != "蓝光" ||
		streams[0].Rate == nil || *streams[0].Rate != 5 {
		t.Errorf("stream[0] = %+v", streams[0])
	}
	if streams[2].URL != "https://pull-flv.douyincdn.com/stage/html-3.flv" {
		t.Errorf("stream[2] = %+v(只解析 flv)", streams[2])
	}
}

func TestResolveLiveStreamsNoRoomID(t *testing.T) {
	reflowCalls := 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "ttwid=warm; Path=/")
		case "/webcast/room/web/enter/":
			w.Write([]byte(`{"status_code":0,"data":{"data":[{}]}}`))
		case "/webcast/room/reflow/info/":
			reflowCalls++
		default:
			// HTML 兜底返回空页
		}
	})
	_, err := resolveLiveStreams(context.Background(), testContext(), "1")
	if err == nil || !strings.Contains(err.Error(), "douyin HTML: 未找到可播流(房间 1") {
		t.Fatalf("err = %v", err)
	}
	if reflowCalls != 0 {
		t.Errorf("无 room 长号不应请求 reflow(%d 次)", reflowCalls)
	}
}

func TestFetchRoomStatusCodeError(t *testing.T) {
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webcast/room/web/enter/" {
			w.Write([]byte(`{"status_code":2483,"prompts":"请登录"}`))
		}
	})
	_, err := fetchRoom(context.Background(), testContext(), "1")
	if err == nil || !strings.Contains(err.Error(), "douyin 内容不可用:请登录(code 2483)") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRoomEmptyBody(t *testing.T) {
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webcast/room/web/enter/" {
			return
		}
	})
	_, err := fetchRoom(context.Background(), testContext(), "1")
	if err == nil || !strings.Contains(err.Error(), "douyin empty body for") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRoomHTTPError(t *testing.T) {
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webcast/room/web/enter/" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	_, err := fetchRoom(context.Background(), testContext(), "1")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v", err)
	}
}

// enter 失败后 reflow 按 TS 语义补抓一次 enter;仍失败再降级 HTML,最终错误是 HTML 层。
func TestResolveLiveStreamsHTMLAfterEnterFailure(t *testing.T) {
	enterCalls, htmlCalls := 0, 0
	withLiveServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Add("Set-Cookie", "ttwid=warm; Path=/")
		case "/webcast/room/web/enter/":
			enterCalls++
			w.WriteHeader(http.StatusInternalServerError)
		case "/1":
			htmlCalls++
		default:
			// 未预期路径
		}
	})
	_, err := resolveLiveStreams(context.Background(), testContext(), "1")
	if err == nil || !strings.Contains(err.Error(), "douyin HTML: 未找到可播流") {
		t.Fatalf("err = %v", err)
	}
	if enterCalls != 2 || htmlCalls != 1 {
		t.Errorf("enter/html 次数 = %d/%d, want 2/1", enterCalls, htmlCalls)
	}
}

func TestResolveLiveStreamsRequiresJS(t *testing.T) {
	sc := testContext()
	sc.JS = nil
	_, err := resolveLiveStreams(context.Background(), sc, "1")
	if !errors.Is(err, platform.ErrJSRunnerUnavailable) {
		t.Fatalf("err = %v", err)
	}
}
