// 拉流解析黄金集:线路归拢顺序、档位过滤(HDR)、ratio 切档(0 不传、已有值覆盖)、
// flv 仅最高档、无档位单流(HLS 优先)与无线路报错。

package huya

import (
	"encoding/json"
	"testing"
)

func TestParseProfile(t *testing.T) {
	lines, rates := parseProfile(loadProfile(t, "profile_room.json"))
	if len(lines) != 3 {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].Cdn != "TX" || lines[0].FlvURL == "" || lines[0].HlsURL == "" {
		t.Errorf("lines[0] = %+v", lines[0])
	}
	if lines[1].Cdn != "HW" || lines[1].FlvURL == "" || lines[1].HlsURL != "" {
		t.Errorf("lines[1] = %+v", lines[1])
	}
	if lines[2].Cdn != "AL" || lines[2].FlvURL != "" || lines[2].HlsURL == "" {
		t.Errorf("lines[2] = %+v", lines[2])
	}
	want := []apiBitRate{{"蓝光10M", 0}, {"蓝光4M", 4000}, {"超清", 2000}, {"流畅", 500}}
	if len(rates) != len(want) {
		t.Fatalf("rates = %+v", rates)
	}
	for i := range want {
		if rates[i] != want[i] {
			t.Errorf("rates[%d] = %+v, want %+v", i, rates[i], want[i])
		}
	}
}

func TestParseBitRateArrayForm(t *testing.T) {
	var res profileRoomResponse
	raw := `{"data":{"liveData":{"bitRateInfo":[{"iBitRate":0,"sDisplayName":"原画"},{"iBitRate":4000,"sDisplayName":"HDR"}]}}}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	rates := parseBitRates(res.Data.LiveData)
	if len(rates) != 1 || rates[0].Name != "原画" || rates[0].BitRate != 0 {
		t.Errorf("rates = %+v", rates)
	}
}

func TestPlayStreams(t *testing.T) {
	streams, err := playStreams("11342412", loadProfile(t, "profile_room.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 10 {
		t.Fatalf("streams = %d, want 10", len(streams))
	}
	first := streams[0]
	if deref(first.Format) != "hls" || deref(first.Quality) != "蓝光10M·TX" || first.Rate == nil || *first.Rate != 0 {
		t.Errorf("streams[0] = %+v", first)
	}
	if first.URL != "https://hls.example/tx.m3u8?wsSecret=1&ratio=0" {
		t.Errorf("ratio=0 不应重写: %s", first.URL)
	}
	if first.Headers["referer"] != "https://m.huya.com/11342412" || first.Headers["user-agent"] != playUA {
		t.Errorf("headers = %+v", first.Headers)
	}
	if deref(streams[1].Format) != "flv" || streams[1].URL != "https://flv.example/tx.flv?wsSecret=1" {
		t.Errorf("streams[1] = %+v", streams[1])
	}
	if streams[2].URL != "https://flv.example/hw.flv?wsSecret=2" || deref(streams[2].Quality) != "蓝光10M·HW" {
		t.Errorf("streams[2] = %+v", streams[2])
	}
	if streams[3].URL != "https://hls.example/al.m3u8" || deref(streams[3].Quality) != "蓝光10M·AL" {
		t.Errorf("streams[3] = %+v", streams[3])
	}
	if streams[4].URL != "https://hls.example/tx.m3u8?wsSecret=1&ratio=4000" || *streams[4].Rate != 4000 {
		t.Errorf("streams[4] = %+v", streams[4])
	}
	if streams[5].URL != "https://hls.example/al.m3u8?ratio=4000" {
		t.Errorf("streams[5] = %+v", streams[5])
	}
	flvCount := 0
	for _, stream := range streams {
		if deref(stream.Format) == "flv" {
			flvCount++
			if stream.Rate == nil || *stream.Rate != 0 {
				t.Errorf("flv 只应产最高档: %+v", stream)
			}
		}
	}
	if flvCount != 2 {
		t.Errorf("flv 数 = %d, want 2", flvCount)
	}
}

func TestPlayStreamsSingleLine(t *testing.T) {
	var res profileRoomResponse
	raw := `{"data":{"stream":{"flv":{"multiLine":[{"cdnType":"TX","url":"https://x/live.flv"}]}}}}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	streams, err := playStreams("1", res.Data)
	if err != nil || len(streams) != 1 {
		t.Fatalf("streams = %+v, err = %v", streams, err)
	}
	if streams[0].URL != "https://x/live.flv" || deref(streams[0].Format) != "flv" {
		t.Errorf("flv 兜底 = %+v", streams[0])
	}
	if streams[0].Headers["referer"] != "https://m.huya.com/1" {
		t.Errorf("headers = %+v", streams[0].Headers)
	}
}

func TestPlayStreamsPrefersHLS(t *testing.T) {
	var res profileRoomResponse
	raw := `{"data":{"stream":{"flv":{"multiLine":[{"cdnType":"TX","url":"https://x/live.flv"}]},"hls":{"multiLine":[{"cdnType":"TX","url":"https://x/live.m3u8"}]}}}}`
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		t.Fatal(err)
	}
	streams, err := playStreams("1", res.Data)
	if err != nil || len(streams) != 1 || deref(streams[0].Format) != "hls" {
		t.Fatalf("streams = %+v, err = %v", streams, err)
	}
}

func TestPlayStreamsNoLineError(t *testing.T) {
	if _, err := playStreams("660000", loadProfile(t, "profile_room_offline.json")); err == nil {
		t.Error("无线路应报错(不能静默返回空)")
	}
}

func TestWithRatio(t *testing.T) {
	cases := []struct {
		url  string
		rate int
		want string
	}{
		{"", 2000, ""},
		{"https://x/a.m3u8", 0, "https://x/a.m3u8"},
		{"https://x/a.m3u8", 2000, "https://x/a.m3u8?ratio=2000"},
		{"https://x/a.m3u8?ws=1", 2000, "https://x/a.m3u8?ws=1&ratio=2000"},
		{"https://x/a.m3u8?ratio=500", 2000, "https://x/a.m3u8?ratio=2000"},
		{"https://x/a.m3u8?ws=1&ratio=500", 2000, "https://x/a.m3u8?ws=1&ratio=2000"},
		{"https://x/a.m3u8?ratio=0", 0, "https://x/a.m3u8?ratio=0"},
	}
	for _, tc := range cases {
		if got := withRatio(tc.url, tc.rate); got != tc.want {
			t.Errorf("withRatio(%q,%d) = %q, want %q", tc.url, tc.rate, got, tc.want)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
