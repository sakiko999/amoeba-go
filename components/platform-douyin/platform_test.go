// 平台装配与播放路由测试:路由表驱动(含 TS 原样例)、Tags/op 面、
// search op 的固定校验。

package douyin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestPlaybackRoutes(t *testing.T) {
	pb := newPlayback(testContext())
	cases := []struct {
		name   string
		raw    string
		wantID string
		wantOK bool
	}{
		{"TS 原样例", "https://live.douyin.com/rid4242", "rid4242", true},
		{"数字短号", "https://live.douyin.com/6753235366", "6753235366", true},
		{"带查询串", "https://live.douyin.com/6753235366?enter_from=web", "6753235366", true},
		{"多段取末段", "https://live.douyin.com/123/456/", "456", true},
		{"百分号编码不 decode", "https://live.douyin.com/%E4%B8%AD%E6%96%87", "%E4%B8%AD%E6%96%87", true},
		{"横杠不是 id", "https://live.douyin.com/-", "", false},
		{"根路径", "https://live.douyin.com/", "", false},
		{"用户主页不命中", "https://www.douyin.com/user/MS4wLjABAAAA", "", false},
		{"相对地址不命中", "live.douyin.com/123", "", false},
		{"普通站点不命中", "https://example.com/123", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, kind, ok := platform.MatchRoute(pb.Routes, tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("MatchRoute(%q) ok = %v, want %v", tc.raw, ok, tc.wantOK)
			}
			if ok && (id != tc.wantID || kind != "live") {
				t.Errorf("MatchRoute(%q) = %q/%q", tc.raw, id, kind)
			}
		})
	}
}

func TestPlaybackDeclaration(t *testing.T) {
	pb := newPlayback(testContext())
	if pb.Platform != "douyin" || len(pb.Routes) != 1 {
		t.Errorf("playback = %+v", pb)
	}
	if pb.ResolveLivePlay == nil || pb.ResolveVideoPlay != nil || pb.GetDanmaku != nil {
		t.Errorf("能力面不完整: %+v", pb)
	}
}

func TestSpecAssembly(t *testing.T) {
	spec := New(testContext())
	if spec.Namespace != "douyin" || spec.Label != "抖音" {
		t.Errorf("namespace/label = %q/%q", spec.Namespace, spec.Label)
	}
	for _, tag := range []string{"platform.data", "search.entity"} {
		if !containsString(spec.Tags, tag) {
			t.Errorf("缺少 tag %q: %v", tag, spec.Tags)
		}
	}
	if containsString(spec.Tags, "search.live") {
		t.Errorf("本阶段不声明 search.live: %v", spec.Tags)
	}
	for _, name := range []string{"channels", "fetch", "play", "search"} {
		if _, ok := spec.Ops[name]; !ok {
			t.Errorf("缺少 op %q", name)
		}
	}
	if _, ok := spec.Ops["danmaku"]; ok {
		t.Error("本阶段不注册 danmaku")
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestSearchOpsValidation(t *testing.T) {
	ops := searchOps(testContext())
	_, err := ops["search"].Invoke(context.Background(), json.RawMessage(`{"keyword":"  ","scope":"entity"}`))
	if err == nil || !strings.Contains(err.Error(), "需要 keyword") {
		t.Errorf("空关键词 err = %v", err)
	}
	_, err = ops["search"].Invoke(context.Background(), json.RawMessage(`{"keyword":"x","scope":"live"}`))
	if err == nil || !strings.Contains(err.Error(), "不支持 scope") {
		t.Errorf("scope 校验 err = %v", err)
	}
}
