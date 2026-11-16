// services 装配测试:全部内置平台注册进网关、自检可调用、占位平台报 PlatformOpError。

package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mygo-host/components"
	"mygo-host/platform"
)

func testContext(t *testing.T) *platform.ServiceContext {
	t.Helper()
	cache, err := platform.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &platform.ServiceContext{
		Logf:  func(string, ...any) {},
		HTTP:  platform.NewHostClient(),
		Cache: cache,
		Creds: platform.NewCredentials(t.TempDir() + "/credentials.json"),
		JS:    platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) { return nil, errors.New("no js") }),
	}
}

func testManager(t *testing.T) *components.Manager {
	t.Helper()
	return components.NewManager(t.TempDir(), t.TempDir())
}

func TestActivateRegistersAllPlatforms(t *testing.T) {
	gw, err := Activate(testContext(t), testManager(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"gateway": false, "tmdb": false, "douban": false, "bangumi": false,
		"apple": false, "youtube": false, "bili": false, "douyu": false,
		"huya": false, "twitch": false,
	}
	for _, info := range gw.List() {
		if _, ok := want[info.Namespace]; !ok {
			t.Errorf("unexpected namespace %q", info.Namespace)
		}
		want[info.Namespace] = true
	}
	for ns, seen := range want {
		if !seen {
			t.Errorf("namespace %q not registered", ns)
		}
	}
}

// 服务与 page 平级登记:网关(根)先激活,平台服务依次入表。
func TestActivateRegistersServicesAsComponents(t *testing.T) {
	comps := testManager(t)
	if _, err := Activate(testContext(t), comps); err != nil {
		t.Fatal(err)
	}
	services := comps.Services()
	if len(services) != 10 {
		t.Fatalf("内置服务应为 10 个(api-gateway + 9 平台),got %d", len(services))
	}
	if services[0].Manifest.ID != "api-gateway" {
		t.Errorf("根服务应为 api-gateway,got %s", services[0].Manifest.ID)
	}
	ids := map[string]bool{}
	for _, s := range services {
		ids[s.Manifest.ID] = true
	}
	for _, want := range []string{"platform-bili", "platform-youtube", "platform-tmdb"} {
		if !ids[want] {
			t.Errorf("服务 %s 未登记", want)
		}
	}
}

func TestGatewayPing(t *testing.T) {
	gw, err := Activate(testContext(t), testManager(t))
	if err != nil {
		t.Fatal(err)
	}
	result, err := gw.Call(context.Background(), "gateway.ping", json.RawMessage(`{"check":1}`))
	if err != nil {
		t.Fatal(err)
	}
	pong, ok := result.(map[string]any)
	if !ok || pong["pong"] != true {
		t.Errorf("ping result = %+v", result)
	}
}

func TestUnknownOpError(t *testing.T) {
	gw, _ := Activate(testContext(t), testManager(t))
	if _, err := gw.Call(context.Background(), "nope.search", nil); err == nil {
		t.Error("expected error for unknown op")
	}
}
