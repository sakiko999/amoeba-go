// Play 参数校验与网关链(services smoke 之外的行为基线)。播放统一走 H5:
// 任何非空流列表都进 H5Target,页面侧再按流类型选引擎。

package player

import (
	"context"
	"encoding/json"
	"testing"

	"mygo-host/platform"
)

type recordingOpener struct {
	title, url, quality string
	h5Open              bool
	h5                  H5Target
	targets             map[string]H5Target
}

func (o *recordingOpener) OpenPlayerH5(title string, target H5Target) error {
	o.h5Open = true
	o.h5 = target
	o.title = title
	return nil
}

func (o *recordingOpener) Target(instanceID string) (H5Target, bool) {
	t, ok := o.targets[instanceID]
	return t, ok
}

func TestPlayParamsValidation(t *testing.T) {
	svc, _, err := NewService(&recordingOpener{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []PlayParams{{}, {Namespace: "twitch"}, {URL: "https://x"}} {
		if err := svc.Play(context.Background(), p); err == nil {
			t.Errorf("Play(%+v) 应报参数错误", p)
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

// 激活后 Play 经网关调用 <ns>.play,流列表原样带进 H5Target(页面侧选引擎)。
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
	err = svc.Play(context.Background(), PlayParams{
		Namespace: "bilibili", URL: "https://www.bilibili.com/video/BV1", Title: "视频", Kind: "video",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gw.op != "bilibili.play" {
		t.Errorf("网关 op = %q, want bilibili.play", gw.op)
	}
	if !opener.h5Open || len(opener.h5.Streams) != 2 || opener.h5.Kind != "video" ||
		opener.h5.Title != "视频" || opener.h5.Namespace != "bilibili" {
		t.Errorf("h5 target = %+v (open=%v)", opener.h5, opener.h5Open)
	}
}

// 平台返回空流列表时报错,不打开播放 tab。
func TestPlayNoStreams(t *testing.T) {
	opener := &recordingOpener{}
	svc, comp, err := NewService(opener)
	if err != nil {
		t.Fatal(err)
	}
	if err := comp.Activate(&fakeGateway{result: platform.PlayOpResult{}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Play(context.Background(), PlayParams{Namespace: "x", URL: "https://x"}); err == nil {
		t.Error("无流应报错")
	}
	if opener.h5Open {
		t.Error("无流不应打开播放 tab")
	}
}

// Target 透传宿主存下的播放目标;没有目标时返回 nil(页面显示空态)。
func TestTargetLookup(t *testing.T) {
	opener := &recordingOpener{targets: map[string]H5Target{
		"player-h5-1": {InstanceID: "player-h5-1", URL: "https://x"},
	}}
	svc, _, err := NewService(opener)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Target(context.Background(), "player-h5-1")
	if err != nil || got == nil || got.URL != "https://x" {
		t.Fatalf("Target = %+v, %v", got, err)
	}
	none, err := svc.Target(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Errorf("没有目标时应为 nil,得到 %+v", none)
	}
}

// 未激活(网关调用面缺失)时 Play 应报错而不是空指针。
func TestPlayRequiresActivation(t *testing.T) {
	svc, _, err := NewService(&recordingOpener{})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Play(context.Background(), PlayParams{Namespace: "twitch", URL: "https://x"}); err == nil {
		t.Error("未激活时 Play 应报错")
	}
}
