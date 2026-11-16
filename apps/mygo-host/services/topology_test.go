// topoSort 的拓扑语义测试:提供者先于消费者、悬空 inject 报错、环报错、
// 无依赖服务保持声明序。

package services

import (
	"strings"
	"testing"

	"mygo-host/platform"
)

type stubService struct {
	platform.ServiceManifest
}

func (s stubService) Manifest() platform.ServiceManifest { return s.ServiceManifest }
func (s stubService) Activate(platform.Registrar) error  { return nil }

func manifest(id string, provides, inject []string) stubService {
	m := platform.ServiceManifest{ID: id, Inject: inject}
	for _, name := range provides {
		m.Provides = append(m.Provides, platform.ProvideDecl{Name: name})
	}
	return stubService{m}
}

func ids(list []platform.Service) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.Manifest().ID
	}
	return out
}

func TestTopoSortProvidersFirst(t *testing.T) {
	// 声明序故意把消费者放前面:网关(a)→平台(p1)/媒体库(media)→二段消费者(p2)。
	ordered, err := topoSort([]platform.Service{
		manifest("media", []string{"media.library"}, []string{"api.gateway"}),
		manifest("p1", nil, []string{"api.gateway"}),
		manifest("gateway", []string{"api.gateway"}, nil),
		manifest("p2", nil, []string{"api.gateway", "media.library"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(ordered)
	if got[0] != "gateway" {
		t.Errorf("根服务应最先,got %v", got)
	}
	if got[1] != "media" || got[3] != "p2" {
		t.Errorf("拓扑序错误(依赖 media 的 p2 应最后):%v", got)
	}
}

func TestTopoSortDanglingInject(t *testing.T) {
	_, err := topoSort([]platform.Service{
		manifest("a", nil, []string{"nope.service"}),
	})
	if err == nil || !strings.Contains(err.Error(), "nope.service") {
		t.Errorf("悬空 inject 应报错,got %v", err)
	}
}

func TestTopoSortCycle(t *testing.T) {
	_, err := topoSort([]platform.Service{
		manifest("a", []string{"svc.b"}, []string{"svc.a"}),
		manifest("b", []string{"svc.a"}, []string{"svc.b"}),
	})
	if err == nil || !strings.Contains(err.Error(), "环") {
		t.Errorf("依赖环应报错,got %v", err)
	}
}

func TestTopoSortStableWithoutDeps(t *testing.T) {
	ordered, err := topoSort([]platform.Service{
		manifest("b", nil, nil), manifest("a", nil, nil), manifest("c", nil, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ids(ordered), ","); got != "b,a,c" {
		t.Errorf("无依赖时应保持声明序,got %s", got)
	}
}

func TestTopoSortDuplicateID(t *testing.T) {
	_, err := topoSort([]platform.Service{
		manifest("a", nil, nil), manifest("a", nil, nil),
	})
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Errorf("重复 id 应报错,got %v", err)
	}
}
