// services 装配测试:全部内置平台注册进网关、自检可调用、占位平台报 PlatformOpError。

package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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

func TestActivateRegistersAllPlatforms(t *testing.T) {
	gw, err := Activate(testContext(t))
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

func TestGatewayPing(t *testing.T) {
	gw, err := Activate(testContext(t))
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
	gw, _ := Activate(testContext(t))
	if _, err := gw.Call(context.Background(), "nope.search", nil); err == nil {
		t.Error("expected error for unknown op")
	}
}
