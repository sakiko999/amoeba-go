// 渠道声明与 op 面对齐 TS tests/platform.test.ts:渠道非空且 key 唯一、
// 声明 defaultInfo 的渠道可零参实例化、passive 渠道必须具备被动采集、
// 播放路由解析;另断言 feed 协议注册的 op 集合与渠道元数据黄金集。

package huya

import (
	"context"
	"testing"

	"mygo-host/platform"
)

func testContext(t *testing.T) *platform.ServiceContext {
	t.Helper()
	return &platform.ServiceContext{
		Logf: func(string, ...any) {},
		HTTP: platform.NewHostClient(),
		Now:  func() int64 { return 1700000000000 },
	}
}

func TestChannelsDeclared(t *testing.T) {
	spec := New(testContext(t))
	result, err := spec.Ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	metas, ok := result.([]platform.ChannelMeta)
	if !ok {
		t.Fatalf("channels result = %T", result)
	}
	if len(metas) == 0 {
		t.Fatal("渠道清单为空")
	}
	seen := map[string]bool{}
	for _, meta := range metas {
		if seen[meta.Key] {
			t.Errorf("重复渠道 key %q", meta.Key)
		}
		seen[meta.Key] = true
		if meta.Name == "" {
			t.Errorf("渠道 %q 名称为空", meta.Key)
		}
		if meta.Passive {
			t.Errorf("渠道 %q 不应为 passive", meta.Key)
		}
	}
}

func TestChannelMetadata(t *testing.T) {
	spec := New(testContext(t))
	result, _ := spec.Ops["channels"].Invoke(context.Background(), nil)
	metas := result.([]platform.ChannelMeta)
	want := []struct {
		key, name, kind string
		tplKey          string
		hasDefault      bool
	}{
		{"live:huya", "虎牙直播房间", "live", "roomIds", false},
		{"live:huya:hot", "虎牙直播热门", "live", "", true},
		{"live:huya:category", "虎牙分区直播", "live", "gameId", false},
		{"live:huya:categories", "虎牙分区列表", "category", "", true},
		{"live:huya:anchor", "虎牙搜主播", "live", "keyword", false},
	}
	if len(metas) != len(want) {
		t.Fatalf("渠道数 = %d, want %d", len(metas), len(want))
	}
	for i, expected := range want {
		meta := metas[i]
		if meta.Key != expected.key || meta.Name != expected.name || meta.Kind != expected.kind {
			t.Errorf("渠道[%d] = %s/%s/%s", i, meta.Key, meta.Name, meta.Kind)
		}
		if expected.tplKey == "" {
			if len(meta.SourceInfoTpl) != 0 {
				t.Errorf("%s 不应声明参数表: %+v", meta.Key, meta.SourceInfoTpl)
			}
		} else {
			if len(meta.SourceInfoTpl) != 1 || meta.SourceInfoTpl[0].Key != expected.tplKey {
				t.Errorf("%s 参数表 = %+v", meta.Key, meta.SourceInfoTpl)
			} else if meta.SourceInfoTpl[0].Required == nil || !*meta.SourceInfoTpl[0].Required {
				t.Errorf("%s 参数应 required", meta.Key)
			}
		}
		if (meta.DefaultInfo != nil) != expected.hasDefault {
			t.Errorf("%s defaultInfo = %v", meta.Key, meta.DefaultInfo)
		}
	}
}

func TestDefaultInfoZeroArgSources(t *testing.T) {
	for _, channel := range channels(testContext(t)) {
		info := channel.DefaultInfo()
		if info == nil {
			continue
		}
		// 等价 TS 的 typeof source.fetch === "function":非 nil 即具备 Fetch。
		if source := channel.GetSource(info); source == nil {
			t.Errorf("渠道 %q 零参实例化得到 nil", channel.Key())
		}
	}
}

func TestNoPassiveChannels(t *testing.T) {
	for _, channel := range channels(testContext(t)) {
		info := channel.DefaultInfo()
		if info == nil {
			info = platform.SourceInfo{}
		}
		if _, ok := channel.GetSource(info).(platform.PassiveSource); ok {
			t.Errorf("渠道 %q 实现了 PassiveSource,但 TS 侧未声明 passive", channel.Key())
		}
	}
}

func TestPlaybackRoute(t *testing.T) {
	pb := playback(testContext(t))
	if pb.Platform != "huya" || pb.ResolveLivePlay == nil || pb.GetDanmaku == nil {
		t.Fatalf("playback 能力不完整: %+v", pb)
	}
	cases := []struct {
		url, id, kind string
		ok            bool
	}{
		{"https://www.huya.com/11342412", "11342412", "live", true},
		{"https://www.huya.com/", "", "", false},
		{"https://example.com/11342412", "", "", false},
		{"https://evilhuya.com/11342412", "", "", false},
	}
	for _, tc := range cases {
		id, kind, ok := platform.MatchRoute(pb.Routes, tc.url)
		if ok != tc.ok || id != tc.id || kind != tc.kind {
			t.Errorf("route(%q) = %q/%q/%v", tc.url, id, kind, ok)
		}
	}
}

func TestOpSurface(t *testing.T) {
	spec := New(testContext(t))
	if spec.Namespace != "huya" || spec.Label != "虎牙" {
		t.Errorf("spec = %s/%s", spec.Namespace, spec.Label)
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "platform.data" {
		t.Errorf("tags = %v", spec.Tags)
	}
	for _, name := range []string{"channels", "fetch", "play", "danmaku"} {
		if _, ok := spec.Ops[name]; !ok {
			t.Errorf("缺少 op %q", name)
		}
	}
	if len(spec.Ops) != 4 {
		t.Errorf("op 数 = %d(应为 feed 协议 4 个)", len(spec.Ops))
	}
}
