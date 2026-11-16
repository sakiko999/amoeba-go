// login/分类归一化与 usher master 解析测试(移植 TS example/twitch-playlist.check.ts)。

package twitch

import (
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestNormalizeTwitchLogin(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"shroud", "shroud", true},
		{"SHROUD", "shroud", true},
		{"@Shroud", "shroud", true},
		{"https://www.twitch.tv/shroud", "shroud", true},
		{"https://twitch.tv/Shroud?ref=x", "shroud", true},
		{"twitch.tv/shroud", "shroud", true},
		{"https://m.twitch.tv/shroud", "shroud", true},
		{"https://www.twitch.tv/%73hroud", "shroud", true},
		{"  shroud  ", "shroud", true},
		{"videos", "", false},
		{"directory", "", false},
		{"search", "", false},
		{"settings", "", false},
		{"https://www.twitch.tv/videos/123", "", false},
		{"https://www.twitch.tv/shroud/clip/ObscureSlug", "", false},
		{"https://www.twitch.tv/", "", false},
		{"https://clips.twitch.tv/FunnySlug", "", false},
		{"clips.twitch.tv/FunnySlug", "", false},
		{"https://player.twitch.tv/shroud", "", false},
		{"https://dashboard.twitch.tv/xyz", "", false},
		{"", "", false},
		{"   ", "", false},
		{"@", "", false},
		{"ab", "", false},
		{strings.Repeat("a", 26), "", false},
		{"shroud!", "", false},
		{"shroud-name", "", false},
		{"sh rou", "", false},
		{"https://example.com/shroud", "", false},
		{"https://nottwitch.tv/shroud", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizeTwitchLogin(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Errorf("normalizeTwitchLogin(%q) = %q/%v, want %q/%v", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestNormalizeTwitchCategory(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"just-chatting", "just-chatting", true},
		{"JUST-CHATTING", "just-chatting", true},
		{"  just-chatting  ", "just-chatting", true},
		{"https://www.twitch.tv/directory/category/just-chatting", "just-chatting", true},
		{"https://www.twitch.tv/directory/category/Just-Chatting?foo=1", "just-chatting", true},
		{"https://www.twitch.tv/directory/category/just-chatting/", "just-chatting", true},
		{"https://m.twitch.tv/directory/category/just-chatting", "just-chatting", true},
		{"twitch.tv/directory/category/just-chatting", "just-chatting", true},
		{"https://www.twitch.tv/directory/category/just%2Dchatting", "just-chatting", true},
		{"", "", false},
		{"   ", "", false},
		{"a", "", false},
		{strings.Repeat("a", 65), "", false},
		{"just_chatting", "", false},
		{"just chatting", "", false},
		{"https://www.twitch.tv/just-chatting", "", false},
		{"https://www.twitch.tv/directory", "", false},
		{"https://www.twitch.tv/directory/category", "", false},
		{"https://www.twitch.tv/directory/game/just-chatting", "", false},
		{"https://www.twitch.tv/directory/category/just-chatting/extra", "", false},
		{"https://www.twitch.tv/directory/category/just%2Fchatting", "", false},
		{"https://example.com/directory/category/just-chatting", "", false},
		{"https://clips.twitch.tv/directory/category/just-chatting", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizeTwitchCategory(tc.input)
		if got != tc.want || ok != tc.ok {
			t.Errorf("normalizeTwitchCategory(%q) = %q/%v, want %q/%v", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRouteOnlyMatchesChannelHosts(t *testing.T) {
	routes := twitchRoutes()
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.twitch.tv/shroud", "shroud"},
		{"https://m.twitch.tv/shroud", "shroud"},
		{"https://www.twitch.tv/%73hroud", "shroud"},
	}
	for _, tc := range cases {
		id, kind, ok := platform.MatchRoute(routes, tc.url)
		if !ok || id != tc.want || kind != "live" {
			t.Errorf("route(%q) = %q/%q/%v, want %q/live", tc.url, id, kind, ok, tc.want)
		}
	}
	for _, url := range []string{
		"https://clips.twitch.tv/FunnySlug",
		"https://www.twitch.tv/shroud/clip/ObscureSlug",
		"https://www.twitch.tv/videos/123",
		"https://nottwitch.tv/shroud",
	} {
		if id, _, ok := platform.MatchRoute(routes, url); ok {
			t.Errorf("route(%q) = %q, want 不命中", url, id)
		}
	}
}

const masterFixture = `#EXTM3U
#EXT-X-TWITCH-INFO:NODE="video-weaver.example",MANIFEST-CLUSTER="example"
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="chunked",NAME="720p60",AUTOSELECT=YES,DEFAULT=YES
#EXT-X-STREAM-INF:BANDWIDTH=3400000,RESOLUTION=1280x720,FRAME-RATE=60.000,CODECS="avc1.4d001f,mp4a.40.2",NAME="720p60"
https://video-weaver.example/hls/720p60/index-dvr.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=160000,CODECS="mp4a.40.2",IVS-NAME="audio_only"
https://video-weaver.example/hls/audio_only/index-dvr.m3u8
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="chunked",NAME="1080p60",AUTOSELECT=YES
#EXT-X-STREAM-INF:BANDWIDTH=6000000,RESOLUTION=1920x1080,FRAME-RATE=60.000,CODECS="avc1.64002a,mp4a.40.2",IVS-NAME="1080p60 (source)"
https://video-weaver.example/hls/1080p60/index-dvr.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=852x480,FRAME-RATE=30.000,CODECS="avc1.4d001e,mp4a.40.2"
https://video-weaver.example/hls/480p30/index-dvr.m3u8
`

func TestParseTwitchMaster(t *testing.T) {
	variants := parseTwitchMaster(masterFixture)
	if len(variants) != 4 {
		t.Fatalf("档位数 = %d, want 4", len(variants))
	}
	if variantHeight(variants[0]) != 1080 {
		t.Errorf("最高档 height = %d, want 1080", variantHeight(variants[0]))
	}
	checkStr(t, "1080p quality", variants[0].Quality, "1080p60 (source)")
	if variants[0].Bandwidth != 6000000 {
		t.Errorf("1080p bandwidth = %d", variants[0].Bandwidth)
	}
	if variants[0].FrameRate == nil || *variants[0].FrameRate != 60 {
		t.Errorf("1080p frameRate = %v", variants[0].FrameRate)
	}
	if variantHeight(variants[1]) != 720 {
		t.Errorf("第二档 height = %d, want 720", variantHeight(variants[1]))
	}
	checkStr(t, "720p quality", variants[1].Quality, "720p60")
	if variantHeight(variants[2]) != 480 {
		t.Errorf("第三档 height = %d, want 480", variantHeight(variants[2]))
	}
	checkStr(t, "480p quality", variants[2].Quality, "480p30")
	if variants[3].Height != nil {
		t.Errorf("audio_only height = %v, want nil", variants[3].Height)
	}
	checkStr(t, "audio_only quality", variants[3].Quality, "audio_only")
	if !strings.Contains(variants[3].URL, "/audio_only/") {
		t.Errorf("audio_only url = %q", variants[3].URL)
	}
	for i := 1; i < len(variants); i++ {
		if variantHeight(variants[i-1]) < variantHeight(variants[i]) {
			t.Errorf("高度未非递增: %d < %d", variantHeight(variants[i-1]), variantHeight(variants[i]))
		}
	}
}

func TestTwitchStreamsFromMaster(t *testing.T) {
	streams, err := twitchStreamsFromMaster(masterFixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 4 {
		t.Fatalf("streams = %d, want 4", len(streams))
	}
	if streams[0].Rate == nil || *streams[0].Rate != 1080 {
		t.Errorf("streams[0].rate = %v, want 1080", streams[0].Rate)
	}
	checkStrPtr(t, "streams[0].format", streams[0].Format, "hls")
	checkStrPtr(t, "streams[0].quality", streams[0].Quality, "1080p60 (source)")
	checkStrPtr(t, "streams[3].quality", streams[3].Quality, "audio_only")
	if streams[3].Rate != nil {
		t.Errorf("audio_only rate = %v, want nil", streams[3].Rate)
	}
	for _, stream := range streams {
		checkStrPtr(t, "stream.format", stream.Format, "hls")
	}
	checkStrPtr(t, "streams[0].url", &streams[0].URL,
		"https://video-weaver.example/hls/1080p60/index-dvr.m3u8")
}

func TestTwitchMasterEmpty(t *testing.T) {
	if variants := parseTwitchMaster(""); len(variants) != 0 {
		t.Errorf("空串档位 = %d", len(variants))
	}
	if variants := parseTwitchMaster("#EXTM3U\n#EXT-X-ENDLIST\n"); len(variants) != 0 {
		t.Errorf("无 STREAM-INF 档位 = %d", len(variants))
	}
	if _, err := twitchStreamsFromMaster("#EXTM3U\n"); err == nil {
		t.Error("无可用档位必须报错")
	}
}
