// 渠道声明与播放路由测试(移植 tests/platform.test.ts)。

package youtube

import (
	"testing"

	"mygo-host/platform"
)

func TestChannelsNonEmptyUnique(t *testing.T) {
	all := channels(testContext(t))
	if len(all) == 0 {
		t.Fatal("channels 为空")
	}
	seen := map[string]bool{}
	for _, channel := range all {
		if seen[channel.Key()] {
			t.Errorf("重复渠道 key %q", channel.Key())
		}
		seen[channel.Key()] = true
		if channel.Name() == "" {
			t.Errorf("渠道 %q 名称为空", channel.Key())
		}
	}
}

func TestChannelsDefaultInfoInstantiable(t *testing.T) {
	for _, channel := range channels(testContext(t)) {
		info := channel.DefaultInfo()
		if info == nil {
			continue
		}
		clone := platform.SourceInfo{}
		for key, value := range info {
			clone[key] = value
		}
		if source := channel.GetSource(clone); source == nil {
			t.Errorf("渠道 %q defaultInfo 无法实例化", channel.Key())
		}
	}
}

// Go DataChannel 契约没有 passive 标志;本平台无被动渠道,等价断言是没有渠道
// 的 source 实现 PassiveSource(协议位保留但未使用)。
func TestChannelsPassiveCapability(t *testing.T) {
	for _, channel := range channels(testContext(t)) {
		source := channel.GetSource(platform.SourceInfo{})
		if _, ok := source.(platform.PassiveSource); ok {
			t.Errorf("渠道 %q 非被动渠道却实现了 PassiveSource", channel.Key())
		}
	}
}

func TestPlaybackRouteWatchURL(t *testing.T) {
	id, kind, ok := platform.MatchRoute(routes(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if !ok || id != "dQw4w9WgXcQ" || kind != "video" {
		t.Errorf("watch 路由 = id %q kind %q ok %v", id, kind, ok)
	}
}

func TestPlaybackRouteShortURL(t *testing.T) {
	id, kind, ok := platform.MatchRoute(routes(), "https://youtu.be/dQw4w9WgXcQ")
	if !ok || id != "dQw4w9WgXcQ" || kind != "video" {
		t.Errorf("youtu.be 路由 = id %q kind %q ok %v", id, kind, ok)
	}
	if _, _, ok := platform.MatchRoute(routes(), "https://example.com/watch?v=x"); ok {
		t.Error("非 youtube host 不应命中路由")
	}
}
