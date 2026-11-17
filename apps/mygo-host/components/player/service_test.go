// pickStream 打分规则与 Play 参数校验(网关链路由 services smoke 覆盖)。

package player

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestPickStream(t *testing.T) {
	cases := []struct {
		name    string
		streams []platform.Stream
		wantURL string // 空 = 无可播
	}{
		{"空列表", nil, ""},
		{"仅 DASH", []platform.Stream{{URL: "d", Format: new("dash")}}, ""},
		{"hls 优先于直连", []platform.Stream{
			{URL: "p", Format: new("progressive")},
			{URL: "h", Format: new("hls")},
		}, "h"},
		{"无 header 直连优先于带 header hls", []platform.Stream{
			{URL: "hh", Format: new("hls"), Headers: map[string]string{"referer": "x"}},
			{URL: "p", Format: new("progressive")},
		}, "p"},
		{"带 header 流兜底", []platform.Stream{
			{URL: "d", Format: new("dash")},
			{URL: "hh", Format: new("hls"), Headers: map[string]string{"referer": "x"}},
		}, "hh"},
		{"未标注 format 按直连", []platform.Stream{{URL: "u"}}, "u"},
	}
	for _, tc := range cases {
		got := pickStream(tc.streams)
		if tc.wantURL == "" {
			if got != nil {
				t.Errorf("%s: 应无可播流,得到 %s", tc.name, got.URL)
			}
			continue
		}
		if got == nil || got.URL != tc.wantURL {
			t.Errorf("%s: want %s, got %+v", tc.name, tc.wantURL, got)
		}
	}
}

type recordingOpener struct{ title, url, quality string }

func (o *recordingOpener) OpenPlayer(title, url, quality string) error {
	o.title, o.url, o.quality = title, url, quality
	return nil
}

func TestPlayParamsValidation(t *testing.T) {
	svc, _, err := NewService(&recordingOpener{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []PlayParams{{}, {Namespace: "twitch"}, {URL: "https://x"}} {
		if _, err := svc.Play(context.Background(), p); err == nil {
			t.Errorf("Play(%+v) 应报参数错误", err)
		}
	}
}

// fakeGateway 同时实现 Registrar 与 OpCaller(与 *gateway.Gateway 同面)。
type fakeGateway struct {
	result any
	err    error
	op     string
}

func (g *fakeGateway) Register(platform.Spec) error { return nil }

func (g *fakeGateway) Call(_ context.Context, op string, _ json.RawMessage) (any, error) {
	g.op = op
	return g.result, g.err
}

// 激活后 Play 经网关调用 <ns>.play,pickStream 选流并交给 Opener。
func TestPlayThroughGateway(t *testing.T) {
	opener := &recordingOpener{}
	svc, comp, err := NewService(opener)
	if err != nil {
		t.Fatal(err)
	}
	gw := &fakeGateway{result: platform.PlayOpResult{ID: "1", Streams: []platform.Stream{
		{URL: "https://cdn/v.mpd", Format: new("dash"), DashManifest: new("<mpd/>")},
		{URL: "https://cdn/v.m3u8", Format: new("hls"), Quality: new("原画")},
	}}}
	if err := comp.Activate(gw); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Play(context.Background(), PlayParams{Namespace: "twitch", URL: "https://www.twitch.tv/x", Title: "直播间"})
	if err != nil {
		t.Fatal(err)
	}
	if gw.op != "twitch.play" {
		t.Errorf("网关 op = %q, want twitch.play", gw.op)
	}
	if opener.url != "https://cdn/v.m3u8" || opener.quality != "原画" || opener.title != "直播间" {
		t.Errorf("opener = %+v", opener)
	}
	if res.Format != "hls" || res.Quality != "原画" {
		t.Errorf("result = %+v", res)
	}
}

// 未激活(网关调用面缺失)时 Play 应报错而不是空指针。
func TestPlayRequiresActivation(t *testing.T) {
	svc, _, err := NewService(&recordingOpener{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Play(context.Background(), PlayParams{Namespace: "twitch", URL: "https://x"}); err == nil {
		t.Error("未激活时 Play 应报错")
	}
}

func TestFmtClock(t *testing.T) {
	cases := []struct {
		d    string
		want string
	}{
		{"0s", "0:00"},
		{"59.4s", "0:59"},
		{"1m1s", "1:01"},
		{"61m", "1:01:00"},
	}
	for _, tc := range cases {
		d, err := time.ParseDuration(tc.d)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmtClock(d); got != tc.want {
			t.Errorf("fmtClock(%s) = %s, want %s", tc.d, got, tc.want)
		}
	}
}
