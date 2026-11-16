// 渠道声明与播放路由测试(移植 TS tests/platform.test.ts)。

package douyu

import (
	"context"
	"testing"

	"mygo-host/platform"
)

func newTestContext() *platform.ServiceContext {
	return &platform.ServiceContext{
		HTTP: platform.NewHostClient(),
		JS: platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) {
			return "signed", nil
		}),
		Now: func() int64 { return 1700000000000 },
	}
}

func TestChannelsDeclaration(t *testing.T) {
	list := channels(newTestContext())
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
		if c.Kind() == "" {
			t.Errorf("渠道 %s kind 为空", c.Key())
		}
	}
}

func TestChannelsWithDefaultInfoInstantiable(t *testing.T) {
	for _, c := range channels(newTestContext()) {
		if c.DefaultInfo() == nil {
			continue
		}
		info := platform.SourceInfo{}
		for k, v := range c.DefaultInfo() {
			info[k] = v
		}
		if source := c.GetSource(info); source == nil {
			t.Errorf("渠道 %s 无法零参实例化", c.Key())
		}
	}
}

func TestChannelsPassiveCapability(t *testing.T) {
	for _, c := range channels(newTestContext()) {
		if _, ok := c.GetSource(platform.SourceInfo{}).(platform.PassiveSource); ok {
			t.Errorf("渠道 %s 声明了被动采集但未实现", c.Key())
		}
	}
}

func TestChannelsOpMeta(t *testing.T) {
	sc := newTestContext()
	ops := platform.FeedOps(platform.FeedSpec{Channels: channels(sc), Playback: newPlayback(sc)})
	result, err := ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	metas := result.([]platform.ChannelMeta)
	wantKeys := []string{"live:douyu", "live:douyu:hot", "live:douyu:category", "live:douyu:categories", "live:douyu:anchor", "douyu:replay"}
	if len(metas) != len(wantKeys) {
		t.Fatalf("channels = %d", len(metas))
	}
	for i, key := range wantKeys {
		if metas[i].Key != key {
			t.Errorf("channels[%d] = %s, want %s", i, metas[i].Key, key)
		}
	}
	if metas[0].SourceInfoTpl[0].Key != "roomIds" || !*metas[0].SourceInfoTpl[0].Required {
		t.Errorf("live sourceInfoTpl = %+v", metas[0].SourceInfoTpl)
	}
	if metas[2].SourceInfoTpl[0].Key != "areaId" || !*metas[2].SourceInfoTpl[0].Required {
		t.Errorf("category sourceInfoTpl = %+v", metas[2].SourceInfoTpl)
	}
	if metas[4].SourceInfoTpl[0].Key != "keyword" || !*metas[4].SourceInfoTpl[0].Required {
		t.Errorf("anchor sourceInfoTpl = %+v", metas[4].SourceInfoTpl)
	}
	if metas[1].DefaultInfo == nil || metas[3].DefaultInfo == nil || metas[0].DefaultInfo != nil {
		t.Errorf("defaultInfo = %v / %v / %v", metas[0].DefaultInfo, metas[1].DefaultInfo, metas[3].DefaultInfo)
	}
	for _, name := range []string{"channels", "fetch", "play", "danmaku"} {
		if _, ok := ops[name]; !ok {
			t.Errorf("缺少 op %s", name)
		}
	}
}

func TestPlaybackRoute(t *testing.T) {
	pb := newPlayback(newTestContext())
	id, kind, ok := platform.MatchRoute(pb.Routes, "https://www.douyu.com/9527")
	if !ok || id != "9527" || kind != "live" {
		t.Errorf("route = %q / %q / %v", id, kind, ok)
	}
	if _, _, ok := platform.MatchRoute(pb.Routes, "https://www.douyu.com/abc"); ok {
		t.Error("非数字房间号不应命中")
	}
	if _, _, ok := platform.MatchRoute(pb.Routes, "https://example.com/9527"); ok {
		t.Error("非斗鱼域名不应命中")
	}
}
