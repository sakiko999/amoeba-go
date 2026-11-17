// 渠道声明与 playback 路由测试(移植 TS tests/platform.test.ts)。

package twitch

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"mygo-host/platform"
)

func testSpec() platform.Spec {
	return New(&platform.ServiceContext{HTTP: platform.NewHostClient()})
}

func channelMetas(t *testing.T) []platform.ChannelMeta {
	t.Helper()
	result, err := testSpec().Ops["channels"].Invoke(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return result.([]platform.ChannelMeta)
}

func TestOpsSurface(t *testing.T) {
	spec := testSpec()
	names := make([]string, 0, len(spec.Ops))
	for name := range spec.Ops {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"channels", "danmaku", "fetch", "play", "search"}
	if len(names) != len(want) {
		t.Fatalf("ops = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("ops = %v, want %v", names, want)
		}
	}
	if spec.Namespace != "twitch" || spec.Label != "Twitch" {
		t.Errorf("spec = %s/%s", spec.Namespace, spec.Label)
	}
	if len(spec.Tags) != 2 || spec.Tags[0] != "platform.data" || spec.Tags[1] != "search.channel" {
		t.Errorf("tags = %v(应含 platform.data + search.channel)", spec.Tags)
	}
	if _, ok := spec.Ops["search"]; !ok {
		t.Errorf("缺少 search op")
	}
}

func TestChannelDeclarations(t *testing.T) {
	metas := channelMetas(t)
	if len(metas) == 0 {
		t.Fatal("channels 为空")
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
	}
	want := []struct {
		key, name, kind string
	}{
		{"twitch:live", "Twitch 直播", "live"},
		{"twitch:vod", "Twitch 直播回放", "video"},
		{"twitch:hot", "Twitch 热门直播", "live"},
		{"twitch:category", "Twitch 分区直播", "live"},
		{"twitch:categories", "Twitch 分区列表", "category"},
		{"twitch:user:search", "Twitch 搜频道", "video"},
	}
	if len(metas) != len(want) {
		t.Fatalf("渠道数 = %d, want %d", len(metas), len(want))
	}
	for i, expected := range want {
		got := metas[i]
		if got.Key != expected.key || got.Name != expected.name || got.Kind != expected.kind {
			t.Errorf("channels[%d] = %s/%s/%s, want %s/%s/%s",
				i, got.Key, got.Name, got.Kind, expected.key, expected.name, expected.kind)
		}
	}
}

func TestChannelDefaultInfoInstantiation(t *testing.T) {
	sc := &platform.ServiceContext{HTTP: platform.NewHostClient()}
	for _, channel := range twitchChannels(sc) {
		info := channel.DefaultInfo()
		if info == nil {
			continue
		}
		if source := channel.GetSource(info); source == nil {
			t.Errorf("渠道 %s 声明了 defaultInfo 但 getSource 为空", channel.Key())
		}
	}
}

func TestPassiveChannelsProvideFetchPassive(t *testing.T) {
	sc := &platform.ServiceContext{HTTP: platform.NewHostClient()}
	for _, channel := range twitchChannels(sc) {
		// twitch 无被动渠道;有 PassiveSource 实现即说明声明了被动能力(接口即保证)。
		if _, ok := channel.GetSource(platform.SourceInfo{}).(platform.PassiveSource); ok {
			t.Errorf("渠道 %s 不应声明被动采集", channel.Key())
		}
	}
}

func TestPlaybackRoutes(t *testing.T) {
	cases := []struct {
		url      string
		id       string
		kind     string
		expected bool
	}{
		{"https://www.twitch.tv/shroud", "shroud", "live", true},
		{"https://m.twitch.tv/%73hroud", "shroud", "live", true},
		{"https://www.twitch.tv/videos/2929172303", "2929172303", "video", true},
		{"https://www.twitch.tv/videos/abc", "", "", false},
		{"https://clips.twitch.tv/FunnySlug", "", "", false},
		{"https://nottwitch.tv/shroud", "", "", false},
	}
	routes := twitchRoutes()
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			id, kind, ok := platform.MatchRoute(routes, tc.url)
			if ok != tc.expected {
				t.Fatalf("match = %v, want %v", ok, tc.expected)
			}
			if !tc.expected {
				return
			}
			if id != tc.id || kind != tc.kind {
				t.Errorf("route = %s/%s, want %s/%s", id, kind, tc.id, tc.kind)
			}
		})
	}
}

// 保证 channels op 结果可 JSON 序列化(字段名与 TS ChannelMeta 一致)。
func TestChannelsJSONShape(t *testing.T) {
	raw, err := json.Marshal(channelMetas(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"key"`, `"name"`, `"kind"`, `"sourceInfoTpl"`, `"required"`, `"placeholder"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("channels JSON 缺少 %s: %s", key, raw)
		}
	}
}
