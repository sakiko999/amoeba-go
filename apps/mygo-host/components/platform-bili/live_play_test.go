// 直播流解析测试（离线）：按实际档位去重（服务端降档时不重复出流）、
// probe 兜底命名与「无可播流必须报错」。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

const livePlayBody = `{"code":0,"data":{"playurl_info":{"playurl":{
	"stream":[{"format":[{"codec":[{
		"codec_name":"avc","base_url":"/live/1.flv","current_qn":250,
		"accept_qn":[10000,400,250],
		"url_info":[{"host":"https://h1.example.com","extra":"?e=1"}]}]}]}],
	"g_qn_desc":[{"qn":10000,"desc":"原画"},{"qn":400,"desc":"蓝光"},{"qn":250,"desc":"超清"}]
}}}}`

func newLiveClient(t *testing.T, body string) *client {
	t.Helper()
	fs := &fakeServer{}
	fs.respond = navResponder(testImgKey, testSubKey, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/frontend/finger/spi") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"b_3":"ANON_B3","b_4":"ANON_B4"}}`), nil
		}
		return jsonResponse(http.StatusOK, body), nil
	})
	return newClient(newTestContext(t, fs, testNow))
}

func TestResolveLivePlayDedupesByAppliedQn(t *testing.T) {
	fs := &fakeServer{}
	fs.respond = navResponder(testImgKey, testSubKey, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, livePlayBody), nil
	})
	c := newClient(newTestContext(t, fs, testNow))
	streams, err := c.resolveLivePlay(context.Background(), "6", platform.ResolveInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 {
		t.Fatalf("服务端统一降到 250，应只剩 1 条流: %d", len(streams))
	}
	s := streams[0]
	if s.URL != "https://h1.example.com/live/1.flv?e=1" || s.Format == nil || *s.Format != "flv" {
		t.Errorf("url/format = %q/%v", s.URL, s.Format)
	}
	if s.Quality == nil || *s.Quality != "超清(请求原画)" || s.Rate == nil || *s.Rate != 250 {
		t.Errorf("quality/rate = %v/%v", s.Quality, s.Rate)
	}
	if s.Headers["referer"] != "https://live.bilibili.com/" || s.Headers["user-agent"] == "" {
		t.Errorf("headers = %v", s.Headers)
	}
	probeOK := false
	for _, r := range fs.requests {
		if strings.Contains(r.URL, "getRoomPlayInfo") && strings.Contains(r.URL, "format=0%2C1%2C2") {
			probeOK = true
		}
	}
	if !probeOK {
		t.Error("probe 应带 format=0,1,2")
	}
}

func TestResolveLivePlayProbeFallback(t *testing.T) {
	body := strings.Replace(livePlayBody, `"accept_qn":[10000,400,250]`, `"accept_qn":[]`, 1)
	c := newLiveClient(t, body)
	streams, err := c.resolveLivePlay(context.Background(), "6", platform.ResolveInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Quality == nil || *streams[0].Quality != "超清" ||
		streams[0].Rate == nil || *streams[0].Rate != 250 {
		t.Errorf("probe 兜底 = %+v", streams)
	}
}

func TestResolveLivePlayNoStreamError(t *testing.T) {
	c := newLiveClient(t, `{"code":0,"data":{}}`)
	_, err := c.resolveLivePlay(context.Background(), "6", platform.ResolveInfo{})
	if err == nil || err.Error() != "bili:live 未找到可播流(房间 6 可能未开播)" {
		t.Fatalf("err = %v", err)
	}
}

func TestParseLiveStreamsOrdering(t *testing.T) {
	// 与 TS R.ascend 一致：非 flv 在前，flv 内非 CDN 在前（mcdn/scdn 最后）。
	body := `{"code":0,"data":{"playurl_info":{"playurl":{"stream":[{"format":[
		{"codec":[{"codec_name":"avc","base_url":"/a.m3u8","url_info":[{"host":"https://h.example.com","extra":""}]}]},
		{"codec":[{"codec_name":"avc","base_url":"/b.flv","url_info":[{"host":"https://mcdn.example.com","extra":""}]}]},
		{"codec":[{"codec_name":"avc","base_url":"/c.flv","url_info":[{"host":"https://h2.example.com","extra":""}]}]}
	]}]}}}}`
	var resp livePlayResp
	if err := platform.UnmarshalJSON([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	streams := parseLiveStreams(resp.Data)
	if len(streams) != 3 {
		t.Fatalf("streams = %d", len(streams))
	}
	want := []string{"https://h.example.com/a.m3u8", "https://h2.example.com/c.flv", "https://mcdn.example.com/b.flv"}
	for i, w := range want {
		if streams[i].URL != w {
			t.Errorf("streams[%d] = %q, want %q", i, streams[i].URL, w)
		}
	}
}
