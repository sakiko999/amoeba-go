// 登录配方与密钥声明的装配契约：平台组件各自导出，装配层只汇总。

package services

import "testing"

func TestLoginFlows(t *testing.T) {
	flows := LoginFlows()
	want := []string{"weibo", "xhs", "douyin", "douban", "bili"}
	if len(flows) != len(want) {
		t.Fatalf("LoginFlows = %d, want %d", len(flows), len(want))
	}
	for i, f := range flows {
		if f.ID != want[i] {
			t.Errorf("flows[%d].ID = %q, want %q", i, f.ID, want[i])
		}
		if f.Label == "" || f.Origin == "" || f.Timeout <= 0 {
			t.Errorf("flow %q 元数据不完整: %+v", f.ID, f)
		}
		if f.HTTPStart == nil && f.Open == nil {
			t.Errorf("flow %q 没有取码路径", f.ID)
		}
	}
}

func TestKeySpecs(t *testing.T) {
	keys := KeySpecs()
	if len(keys) != 2 {
		t.Fatalf("KeySpecs = %d, want 2", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if k.Name == "" || k.Label == "" || seen[k.Name] {
			t.Errorf("key spec 不合法或重复: %+v", k)
		}
		seen[k.Name] = true
	}
}
