// 视频播放解析测试（离线）：DASH 优先 + 每档 MPD、durl 回退逐档、章节兜底。
// 上游 playurl 带 deadline 签名，这里只验证「选流/命名/装配」语义。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

const dashPlayurlBody = `{"code":0,"data":{
	"accept_quality":[80,64],"accept_description":["1080P","720P"],
	"dash":{"duration":123.456,"minBufferTime":1.5,
		"video":[
			{"id":80,"baseUrl":"https://x/v80.m4s","width":1920,"height":1080,
			 "codecs":"avc1.640033","bandwidth":1000000,
			 "SegmentBase":{"Initialization":"0-100","indexRange":"101-200"}},
			{"id":64,"baseUrl":"https://x/v64.m4s","width":1280,"height":720,
			 "codecs":"hev1.1.6.L120.90","bandwidth":900000}],
		"audio":[{"id":30280,"baseUrl":"https://x/a.m4s","codecs":"mp4a.40.2","bandwidth":128000}]}}}`

func playurlResponder(body string) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		url := req.URL.String()
		switch {
		case strings.Contains(url, "/x/web-interface/view"):
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"cid":123,"duration":123.456}}`), nil
		case strings.Contains(url, "/x/player/playurl"):
			return jsonResponse(http.StatusOK, body), nil
		case strings.Contains(url, "/x/player/v2"):
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"view_points":[
				{"from":30,"content":" 正片 "},{"from":0,"content":"开场"},{"from":10,"content":"  "}
			]}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
		}
	}
}

func TestResolveVideoPlayDash(t *testing.T) {
	fs := &fakeServer{respond: playurlResponder(dashPlayurlBody)}
	c := newClient(newTestContext(t, fs, testNow))
	result, err := c.resolveVideoPlay(context.Background(), "BV1xx411c7mD", platform.ResolveInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Streams) != 1 {
		t.Fatalf("HEVC 应被过滤，只剩 avc: %d", len(result.Streams))
	}
	s := result.Streams[0]
	if s.Format == nil || *s.Format != "dash" || s.URL != "" {
		t.Errorf("format/url = %v/%q", s.Format, s.URL)
	}
	if s.Quality == nil || *s.Quality != "1080P" || s.Rate == nil || *s.Rate != 80 {
		t.Errorf("quality/rate = %v/%v", s.Quality, s.Rate)
	}
	if s.DashManifest == nil {
		t.Fatal("缺 dashManifest")
	}
	for _, want := range []string{
		`mediaPresentationDuration="PT123.456S"`,
		`<Representation id="80" mimeType="video/mp4" codecs="avc1.640033" bandwidth="1000000" width="1920" height="1080">`,
		`<BaseURL>https://x/v80.m4s</BaseURL>`,
		`<SegmentBase indexRange="101-200">`,
		`<Initialization range="0-100" />`,
		`<BaseURL>https://x/a.m4s</BaseURL>`,
	} {
		mustContain(t, *s.DashManifest, want, "MPD")
	}

	if len(result.Chapters) != 2 {
		t.Fatalf("chapters = %+v（空 label 应跳过）", result.Chapters)
	}
	if result.Chapters[0].T != 0 || result.Chapters[0].Label != "开场" ||
		result.Chapters[1].T != 30 || result.Chapters[1].Label != "正片" {
		t.Errorf("chapters = %+v", result.Chapters)
	}
	mustContain(t, fs.find("playurl").URL, "bvid=BV1xx411c7mD", "bvid 参数")
	mustContain(t, fs.find("playurl").URL, "cid=123", "cid 参数")
}

func TestResolveVideoPlayDurlFallback(t *testing.T) {
	durlBody := `{"code":0,"data":{
		"accept_quality":[80,64],"accept_description":["1080P","720P"],
		"durl":[{"url":"https://x/v.mp4"}]}}`
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		url := req.URL.String()
		switch {
		case strings.Contains(url, "/x/web-interface/view"):
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"cid":123}}`), nil
		case strings.Contains(url, "fnval=16"):
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"dash":null}}`), nil
		case strings.Contains(url, "/x/player/playurl"):
			return jsonResponse(http.StatusOK, durlBody), nil
		default:
			return jsonResponse(http.StatusOK, `{"code":-1,"message":"no"}`), nil
		}
	}}
	c := newClient(newTestContext(t, fs, testNow))
	result, err := c.resolveVideoPlay(context.Background(), "av170001", platform.ResolveInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chapters) != 0 {
		t.Errorf("章节接口失败应为空: %+v", result.Chapters)
	}
	if len(result.Streams) != 2 {
		t.Fatalf("durl streams = %d", len(result.Streams))
	}
	if *result.Streams[0].Rate != 80 || *result.Streams[0].Quality != "1080P" ||
		*result.Streams[1].Rate != 64 || *result.Streams[1].Quality != "720P" {
		t.Errorf("降序档位 = %+v", result.Streams)
	}
	if result.Streams[0].Format == nil || *result.Streams[0].Format != "mp4" ||
		result.Streams[0].URL != "https://x/v.mp4" {
		t.Errorf("durl stream = %+v", result.Streams[0])
	}
	mustContain(t, fs.find("playurl").URL, "avid=170001", "playurl 的 av 号必须用 avid")
}

func TestViewIDParam(t *testing.T) {
	cases := map[string]string{
		"BV1xx411c7mD": "bvid=BV1xx411c7mD",
		"av170001":     "aid=170001",
	}
	for id, want := range cases {
		if got := viewIDParam(id); got != want {
			t.Errorf("viewIDParam(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestResolveCidMissing(t *testing.T) {
	fs := &fakeServer{}
	c := newClient(newTestContext(t, fs, testNow))
	_, err := c.resolveCid(context.Background(), "BV1xx411c7mD", nil)
	if err == nil || err.Error() != "bilibili view: no cid for BV1xx411c7mD" {
		t.Fatalf("err = %v", err)
	}
}
