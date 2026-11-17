// pickStream 打分规则与 Play 参数校验(网关链路由 services smoke 覆盖)。

package player

import (
	"context"
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
	svc := NewService(nil, &recordingOpener{})
	for _, p := range []PlayParams{{}, {Namespace: "twitch"}, {URL: "https://x"}} {
		if _, err := svc.Play(context.Background(), p); err == nil {
			t.Errorf("Play(%+v) 应报参数错误", p)
		}
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
