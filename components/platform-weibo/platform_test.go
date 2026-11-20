// 平台装配面测试:manifest、tags/scope 与载具缺失降级。

package weibo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestComponentManifest(t *testing.T) {
	svc, err := NewService(&platform.ServiceContext{})
	if err != nil {
		t.Fatal(err)
	}
	m := svc.Manifest()
	if m.ID != "platform-weibo" || m.Kind != "service" || m.Name != "微博" {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Inject) != 1 || m.Inject[0] != "api.gateway" {
		t.Errorf("inject = %v, want [api.gateway]", m.Inject)
	}
}

func TestSpecSurface(t *testing.T) {
	spec := New(&platform.ServiceContext{})
	if spec.Namespace != "weibo" || spec.Label != "微博" {
		t.Errorf("namespace/label = %s/%s", spec.Namespace, spec.Label)
	}
	if len(spec.Tags) != 2 || spec.Tags[0] != "platform.data" || spec.Tags[1] != "search.entity" {
		t.Errorf("tags = %v, want platform.data + search.entity", spec.Tags)
	}
	if len(spec.Ops) != 1 {
		t.Fatalf("ops = %d 项, want 仅 search", len(spec.Ops))
	}
	if _, ok := spec.Ops["search"]; !ok {
		t.Fatal("缺少 search op")
	}
}

func TestSearchRejectsLiveScope(t *testing.T) {
	op := New(&platform.ServiceContext{}).Ops["search"]
	_, err := op.Invoke(context.Background(), json.RawMessage(`{"keyword":"x","scope":"live"}`))
	if err == nil || !strings.Contains(err.Error(), "不支持 scope") {
		t.Fatalf("err = %v, want scope 校验失败", err)
	}
}

func TestSearchWithoutBrowser(t *testing.T) {
	op := New(&platform.ServiceContext{}).Ops["search"]
	_, err := op.Invoke(context.Background(), json.RawMessage(`{"keyword":"半佛仙人","scope":"entity"}`))
	if !errors.Is(err, platform.ErrBrowserUnavailable) {
		t.Fatalf("err = %v, want ErrBrowserUnavailable", err)
	}
}
