// 直播流解析测试:SDK/兜底两种 stream_url 形状、清晰度排序、headers 与
// IPv6 过滤;fixture 来自 testdata。

package douyin

import (
	"encoding/json"
	"testing"

	"mygo-host/platform"
)

func TestParseReflowStreams(t *testing.T) {
	var body reflowResponse
	if err := json.Unmarshal(fixture(t, "reflow_room.json"), &body); err != nil {
		t.Fatal(err)
	}
	assertReflowStreams(t, parseReflowStreams(body.streamURL()))
}

func TestParseHTMLPullStreams(t *testing.T) {
	streams := parseHTMLPullStreams(string(fixture(t, "live_page.html")))
	if len(streams) != 3 {
		t.Fatalf("streams = %+v", streams)
	}
	if streams[0].URL != "https://pull-flv.douyincdn.com/stage/html-5.flv?a=1&b=2" ||
		streams[0].Quality == nil || *streams[0].Quality != "蓝光" ||
		streams[0].Rate == nil || *streams[0].Rate != 5 ||
		streams[0].Format == nil || *streams[0].Format != "flv" {
		t.Errorf("stream[0] = %+v", streams[0])
	}
	if streams[0].Headers["authority"] != "live.douyin.com" {
		t.Errorf("headers = %+v", streams[0].Headers)
	}
}

func TestParseStreamsSDK(t *testing.T) {
	streams := parseStreams(fixture(t, "stream_sdk.json"))
	if len(streams) != 2 {
		t.Fatalf("streams = %+v", streams)
	}
	hd := streams[0]
	if hd.URL != "https://pull-flv.douyincdn.com/stage/stream-3.flv" || hd.Format == nil || *hd.Format != "flv" {
		t.Errorf("hd = %+v", hd)
	}
	if hd.Quality == nil || *hd.Quality != "hd" || hd.Rate == nil || *hd.Rate != 3 {
		t.Errorf("hd quality/rate = %v/%v(空 name 应回退 sdk_key)", hd.Quality, hd.Rate)
	}
	if hd.Headers["authority"] != liveOrigin || hd.Headers["referer"] != liveOrigin || hd.Headers["user-agent"] != uaEnter {
		t.Errorf("hd headers = %+v", hd.Headers)
	}
	sd := streams[1]
	if sd.URL != "https://pull-hls.douyincdn.com/stage/stream-1.m3u8" || sd.Format == nil || *sd.Format != "hls" {
		t.Errorf("sd = %+v", sd)
	}
	if sd.Quality == nil || *sd.Quality != "标清" || sd.Rate == nil || *sd.Rate != 1 {
		t.Errorf("sd quality/rate = %v/%v", sd.Quality, sd.Rate)
	}
	if _, ok := sd.Headers["authority"]; ok {
		t.Errorf("SDK hls 不应带 authority: %+v", sd.Headers)
	}
}

func TestParseStreamsPullURLs(t *testing.T) {
	streams := parseStreams(fixture(t, "stream_pull_urls.json"))
	if len(streams) != 2 {
		t.Fatalf("streams = %+v", streams)
	}
	first := streams[0]
	if first.URL != "https://pull-flv.douyincdn.com/stage/stream-5.flv" || first.Format == nil || *first.Format != "flv" ||
		first.Quality == nil || *first.Quality != "蓝光" || first.Rate == nil || *first.Rate != 4 {
		t.Errorf("stream[0] = %+v", first)
	}
	second := streams[1]
	if second.URL != "https://pull-hls.douyincdn.com/stage/stream-2.m3u8" || second.Format == nil || *second.Format != "hls" ||
		second.Quality == nil || *second.Quality != "流畅" || second.Rate == nil || *second.Rate != 1 {
		t.Errorf("stream[1] = %+v(flv 为空应回落 hls 倒序索引)", second)
	}
	if _, ok := second.Headers["authority"]; ok {
		t.Errorf("兜底路径不应带 authority: %+v", second.Headers)
	}
}

func TestParseStreamsMalformedFallsBack(t *testing.T) {
	raw := []byte(`{"live_core_sdk_data":{"pull_data":{"stream_data":"{bad","options":{"qualities":[{"sdk_key":"HD1","level":1}]}}},"flv_pull_url":{"HD1":"https://x.test/hd.flv"}}`)
	streams := parseStreams(raw)
	if len(streams) != 1 || streams[0].URL != "https://x.test/hd.flv" {
		t.Fatalf("streams = %+v", streams)
	}
	if streams[0].Rate == nil || *streams[0].Rate != 1 {
		t.Errorf("rate = %v", streams[0].Rate)
	}
}

func TestParseStreamsEmpty(t *testing.T) {
	if streams := parseStreams(nil); len(streams) != 0 {
		t.Fatalf("streams = %+v", streams)
	}
}

func TestFilterPlayable(t *testing.T) {
	streams := []platform.Stream{
		{URL: "http://[2408::1]/x.flv"},
		{URL: "https://ok.test/x.flv"},
		{URL: "http://[2001:db8::10]/y.flv"},
	}
	out := filterPlayable(streams)
	if len(out) != 1 || out[0].URL != "https://ok.test/x.flv" {
		t.Fatalf("out = %+v", out)
	}
}
