// 移植 TS tests/platform.test.ts：渠道声明（非空/唯一/可零参实例化/无被动采集）
// 与 playback URL 路由；另加 channels op 的元数据对照（16 个渠道全量 key/kind/参数表）。

package bili

import (
	"context"
	"testing"

	"mygo-host/platform"
)

func testChannels(t *testing.T) []platform.DataChannel {
	t.Helper()
	return channels(newClient(newTestContext(t, &fakeServer{}, testNow)))
}

func TestChannelsDeclaration(t *testing.T) {
	list := testChannels(t)
	if len(list) == 0 {
		t.Fatal("渠道为空")
	}
	seen := map[string]bool{}
	for _, c := range list {
		if seen[c.Key()] {
			t.Errorf("渠道 key 重复: %s", c.Key())
		}
		seen[c.Key()] = true
		if c.Name() == "" {
			t.Errorf("渠道 %s 名称为空", c.Key())
		}
	}
}

func TestChannelsWithDefaultInfoInstantiable(t *testing.T) {
	for _, c := range testChannels(t) {
		if c.DefaultInfo() == nil {
			continue
		}
		info := platform.SourceInfo{}
		for k, v := range c.DefaultInfo() {
			info[k] = v
		}
		if c.GetSource(info) == nil {
			t.Errorf("渠道 %s 无法零参实例化", c.Key())
		}
	}
}

func TestChannelsPassiveCapability(t *testing.T) {
	for _, c := range testChannels(t) {
		if _, ok := c.GetSource(platform.SourceInfo{}).(platform.PassiveSource); ok {
			t.Errorf("渠道 %s 声明了被动采集但未实现", c.Key())
		}
	}
}

func TestChannelsOpMeta(t *testing.T) {
	c := newClient(newTestContext(t, &fakeServer{}, testNow))
	ops := platform.FeedOps(platform.FeedSpec{Channels: channels(c), Playback: newPlayback(c)})
	result, err := ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	metas := result.([]platform.ChannelMeta)
	want := []struct {
		key  string
		kind platform.Kind
	}{
		{"bili:square", platform.KindSocial},
		{"bili:popular", platform.KindVideo},
		{"bili:precious", platform.KindVideo},
		{"bili:ranking", platform.KindVideo},
		{"bili:weekly", platform.KindVideo},
		{"bili:user_video", platform.KindVideo},
		{"bili:live", platform.KindLive},
		{"bili:live:hot", platform.KindLive},
		{"bili:live:areas", platform.KindCategory},
		{"bili:live:area", platform.KindLive},
		{"bili:season", platform.KindVideo},
		{"bili:series", platform.KindVideo},
		{"bili:favlist", platform.KindVideo},
		{"bili:dynamic", platform.KindSocial},
		{"bili:anchor", platform.KindLive},
		{"bili:user", platform.KindVideo},
	}
	if len(metas) != len(want) {
		t.Fatalf("channels = %d, want %d", len(metas), len(want))
	}
	for i, w := range want {
		if metas[i].Key != w.key || metas[i].Kind != string(w.kind) {
			t.Errorf("channels[%d] = %s/%s, want %s/%s", i, metas[i].Key, metas[i].Kind, w.key, w.kind)
		}
	}
	if metas[3].SourceInfoTpl[0].Key != "rid" || metas[3].SourceInfoTpl[0].Required == nil || *metas[3].SourceInfoTpl[0].Required {
		t.Errorf("ranking rid 应为显式非必填: %+v", metas[3].SourceInfoTpl[0])
	}
	if metas[5].SourceInfoTpl[0].Key != "uid" || !*metas[5].SourceInfoTpl[0].Required {
		t.Errorf("user_video uid 应必填: %+v", metas[5].SourceInfoTpl[0])
	}
	if len(metas[6].SourceInfoTpl) != 2 || !*metas[6].SourceInfoTpl[0].Required ||
		*metas[6].SourceInfoTpl[1].Required {
		t.Errorf("live 参数表 = %+v", metas[6].SourceInfoTpl)
	}
	if metas[8].DefaultInfo == nil {
		t.Error("bili:live:areas 应声明 defaultInfo")
	}
	if metas[9].SourceInfoTpl[1].Placeholder == nil || *metas[9].SourceInfoTpl[1].Placeholder != "0" ||
		metas[9].SourceInfoTpl[1].Required != nil {
		t.Errorf("live:area areaId 参数 = %+v", metas[9].SourceInfoTpl[1])
	}
	if metas[15].SourceInfoTpl[0].Placeholder == nil || *metas[15].SourceInfoTpl[0].Placeholder != "如:半佛仙人" {
		t.Errorf("user keyword 参数 = %+v", metas[15].SourceInfoTpl[0])
	}
	for _, name := range []string{"channels", "fetch", "play", "danmaku"} {
		if _, ok := ops[name]; !ok {
			t.Errorf("缺少 op %s", name)
		}
	}
}

func TestPlaybackRoutes(t *testing.T) {
	pb := newPlayback(newClient(newTestContext(t, &fakeServer{}, testNow)))
	cases := []struct {
		url  string
		id   string
		kind string
	}{
		{"https://www.bilibili.com/video/BV1xx411c7mD", "BV1xx411c7mD", "video"},
		{"https://www.bilibili.com/video/av170001", "av170001", "video"},
		{"https://live.bilibili.com/6", "6", "live"},
	}
	for _, c := range cases {
		id, kind, ok := platform.MatchRoute(pb.Routes, c.url)
		if !ok || id != c.id || kind != c.kind {
			t.Errorf("route(%s) = %q/%q/%v, want %q/%q", c.url, id, kind, ok, c.id, c.kind)
		}
	}
	if _, _, ok := platform.MatchRoute(pb.Routes, "https://example.com/video/BV1xx411c7mD"); ok {
		t.Error("非 B 站域名不应命中")
	}
	if _, _, ok := platform.MatchRoute(pb.Routes, "https://live.bilibili.com/abc"); ok {
		t.Error("非数字房间号不应命中")
	}
}

func TestGetDanmakuDefaultsToVod(t *testing.T) {
	sc := newTestContext(t, &fakeServer{}, testNow)
	sc.Logf = func(string, ...any) {} // 订阅是异步 goroutine，测试结束后不能再 t.Logf
	c := newClient(sc)
	pb := newPlayback(c)
	if pb.GetDanmaku == nil {
		t.Fatal("GetDanmaku 未装配")
	}
	// 只断言 dispatch 不 panic：VOD 路径订阅时才会发请求，随后取消。
	subscribe := pb.GetDanmaku("BV1xx411c7mD", platform.DanmakuInfo{})
	cancel := subscribe(func([]platform.DanmakuItem) {})
	cancel()
}
