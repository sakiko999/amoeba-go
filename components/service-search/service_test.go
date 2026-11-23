// Search 服务测试:网关用假平台 Spec 注册(fast/slow/fail/仅 entity/无 search),
// 直调 run/Sources/Query(nil channel)验证发现、fan-out、错误隔离与取消。

package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"mygo-host/components/service-api-gateway"
	"mygo-host/platform"
)

func item(id string) platform.Item {
	return platform.Item{ID: id, Title: id, Kind: platform.KindLive, FetchedAt: 1}
}

// newSvc 走与宿主相同的两步构造:NewService 拿组件包装,Activate 注入网关。
func newSvc(t *testing.T, gw *gateway.Gateway) *Service {
	t.Helper()
	svc, comp, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	if err := comp.Activate(gw); err != nil {
		t.Fatal(err)
	}
	return svc
}

func stubOps(search platform.SearchSpec) platform.Ops {
	return platform.SearchOps(search)
}

func fakeGateway(t *testing.T) *gateway.Gateway {
	t.Helper()
	gw := gateway.New(func(string, ...any) {})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(gw.Register(platform.Spec{
		Namespace: "fast", Label: "快平台",
		Tags: []string{"platform.data", "search.live", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				return platform.FetchPage{Items: []platform.Item{item("f1"), item("f2")}}, nil
			},
		}),
	}))
	must(gw.Register(platform.Spec{
		Namespace: "slow", Label: "慢平台",
		Tags: []string{"platform.data", "search.live", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				select {
				case <-time.After(50 * time.Millisecond):
					return platform.FetchPage{Items: []platform.Item{item("s1")}}, nil
				case <-ctx.Done():
					return platform.FetchPage{}, ctx.Err()
				}
			},
		}),
	}))
	must(gw.Register(platform.Spec{
		Namespace: "failer", Label: "失败平台",
		Tags: []string{"platform.data", "search.live", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				return platform.FetchPage{}, errors.New("上游 500")
			},
		}),
	}))
	must(gw.Register(platform.Spec{
		Namespace: "chonly", Label: "仅频道",
		Tags: []string{"platform.data", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				return platform.FetchPage{Items: []platform.Item{item("c1")}}, nil
			},
		}),
	}))
	must(gw.Register(platform.Spec{
		Namespace: "meta", Label: "元数据平台",
		Tags: []string{"media.metadata"},
		Ops: platform.Ops{"search": {
			Description: "元数据搜索(垂直域,不该被聚合发现)",
			Invoke:      func(ctx context.Context, raw json.RawMessage) (any, error) { return nil, nil },
		}},
	}))
	return gw
}

func TestSourcesDiscovery(t *testing.T) {
	svc := newSvc(t, fakeGateway(t))
	live, err := svc.Sources(context.Background(), platform.SearchLive)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 3 || live[0].Namespace != "failer" || live[1].Namespace != "fast" || live[2].Namespace != "slow" {
		t.Errorf("live sources = %+v, want failer/fast/slow(按 namespace 序)", live)
	}
	channel, _ := svc.Sources(context.Background(), platform.SearchEntity)
	if len(channel) != 4 || !strings.Contains(strings.Join(names(channel), ","), "chonly") {
		t.Errorf("channel sources = %v", names(channel))
	}
}

func names(sources []Source) []string {
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		out = append(out, s.Namespace)
	}
	return out
}

func TestRunFanoutAndErrors(t *testing.T) {
	svc := newSvc(t, fakeGateway(t))
	sources, _ := svc.Sources(context.Background(), platform.SearchEntity)
	var emitted []PlatformGroup
	result := svc.run(context.Background(),
		QueryParams{Keyword: "x", Scope: platform.SearchEntity}, sources,
		func(g PlatformGroup) error {
			emitted = append(emitted, g)
			return nil
		})
	if len(result.Groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(result.Groups))
	}
	byNS := map[string]PlatformGroup{}
	for _, g := range result.Groups {
		byNS[g.Namespace] = g
	}
	if len(byNS["fast"].Items) != 2 || byNS["fast"].Items[0].ID != "f1" {
		t.Errorf("fast group = %+v", byNS["fast"])
	}
	if byNS["failer"].Error == nil || byNS["failer"].Error.Code != "platform" || !strings.Contains(byNS["failer"].Error.Message, "500") {
		t.Errorf("failer error = %+v(guard 包装后应为 platform)", byNS["failer"].Error)
	}
	if byNS["failer"].Items == nil || len(byNS["failer"].Items) != 0 {
		t.Errorf("失败组 items 应为非 nil 空切片")
	}
	if len(emitted) != 4 {
		t.Errorf("emit 次数 = %d, want 4", len(emitted))
	}
}

// Result.Groups 按声明顺序(慢平台后完成也排在 Sources 位置)。
func TestRunGroupOrder(t *testing.T) {
	svc := newSvc(t, fakeGateway(t))
	sources, _ := svc.Sources(context.Background(), platform.SearchLive)
	result := svc.run(context.Background(), QueryParams{Keyword: "x", Scope: platform.SearchLive}, sources, nil)
	want := []string{"failer", "fast", "slow"}
	for i, ns := range want {
		if result.Groups[i].Namespace != ns {
			t.Errorf("groups[%d] = %s, want %s", i, result.Groups[i].Namespace, ns)
		}
	}
}

func TestRunEmitErrorCancelsRest(t *testing.T) {
	svc := newSvc(t, fakeGateway(t))
	sources, _ := svc.Sources(context.Background(), platform.SearchLive)
	var seen int
	result := svc.run(context.Background(), QueryParams{Keyword: "x", Scope: platform.SearchLive}, sources,
		func(g PlatformGroup) error {
			seen++
			return errors.New("page stopped listening") // 首组后停听
		})
	if seen != 1 {
		t.Errorf("emit 次数 = %d, want 1(停听后不再推)", seen)
	}
	canceled := 0
	for _, g := range result.Groups {
		if g.Error != nil && g.Error.Code == "canceled" {
			canceled++
		}
	}
	if canceled == 0 {
		t.Errorf("停听应取消其余平台,groups = %v", result.Groups)
	}
}

func TestRunTimeout(t *testing.T) {
	old := perPlatformTimeout
	perPlatformTimeout = 20 * time.Millisecond
	t.Cleanup(func() { perPlatformTimeout = old })
	svc := newSvc(t, fakeGateway(t))
	sources, _ := svc.Sources(context.Background(), platform.SearchLive)
	result := svc.run(context.Background(), QueryParams{Keyword: "x", Scope: platform.SearchLive}, sources, nil)
	byNS := map[string]PlatformGroup{}
	for _, g := range result.Groups {
		byNS[g.Namespace] = g
	}
	if byNS["slow"].Error == nil || byNS["slow"].Error.Code != "timeout" {
		t.Errorf("slow 应超时, got %+v", byNS["slow"].Error)
	}
	if len(byNS["fast"].Items) != 2 {
		t.Errorf("fast 不应受慢平台影响")
	}
}

// 浏览器载具平台必须用独立的更大超时,普通平台维持 8s。
func TestPlatformTimeoutOverride(t *testing.T) {
	for _, ns := range []string{"xhs", "weibo", "douyin"} {
		if got := platformTimeout(ns); got != browserPlatformTimeout {
			t.Errorf("platformTimeout(%q) = %v, want %v", ns, got, browserPlatformTimeout)
		}
	}
	if got := platformTimeout("bili"); got != perPlatformTimeout {
		t.Errorf("platformTimeout(bili) = %v, want %v", got, perPlatformTimeout)
	}
}

func TestQueryValidationAndFilter(t *testing.T) {
	svc := newSvc(t, fakeGateway(t))
	if _, err := svc.Query(context.Background(), QueryParams{Keyword: " ", Scope: platform.SearchLive}, nil); err == nil {
		t.Error("空 keyword 应报错")
	}
	result, err := svc.Query(context.Background(), QueryParams{Keyword: "x", Scope: "video"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scope != platform.SearchEntity {
		t.Errorf("scope = %q, want entity(旧参数被统一搜索覆盖)", result.Scope)
	}
	if len(result.Groups) != 4 {
		t.Errorf("groups = %d, want 4 个 entity 平台", len(result.Groups))
	}
	// nil channel:非流式,单平台过滤。
	filtered, err := svc.Query(context.Background(),
		QueryParams{Keyword: "x", Scope: platform.SearchEntity, Platforms: []string{"chonly"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Groups) != 1 || filtered.Groups[0].Namespace != "chonly" || len(filtered.Groups[0].Items) != 1 {
		t.Errorf("filtered result = %+v", filtered.Groups)
	}
	raw, err := json.Marshal(filtered)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"items":[`) {
		t.Errorf("items 应序列化为非 null 数组: %s", raw)
	}
}

// 全程成功的结果进缓存:同 keyword 第二次查询不再打平台。
func TestQueryCacheHit(t *testing.T) {
	calls := 0
	gw := gateway.New(func(string, ...any) {})
	if err := gw.Register(platform.Spec{
		Namespace: "fast", Label: "快平台",
		Tags: []string{"platform.data", "search.live", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchLive, platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				calls++
				return platform.FetchPage{Items: []platform.Item{item("f1")}}, nil
			},
		}),
	}); err != nil {
		t.Fatal(err)
	}
	svc := newSvc(t, gw)
	p := QueryParams{Keyword: "缓存"}
	first, err := svc.Query(context.Background(), p, nil)
	if err != nil || calls != 1 {
		t.Fatalf("first: %v, calls=%d", err, calls)
	}
	second, err := svc.Query(context.Background(), p, nil)
	if err != nil || calls != 1 {
		t.Fatalf("缓存命中不应再打平台: calls=%d, %v", calls, err)
	}
	if len(second.Groups) != 1 || len(first.Groups) != 1 ||
		second.Groups[0].Items[0].ID != first.Groups[0].Items[0].ID {
		t.Errorf("缓存快照不一致: %+v", second.Groups)
	}
}

// 带失败组的快照不缓存:失败是暂态,下一次查询必须重打平台。
func TestQueryCacheSkipsFailed(t *testing.T) {
	calls := 0
	gw := gateway.New(func(string, ...any) {})
	if err := gw.Register(platform.Spec{
		Namespace: "flaky", Label: "抖动平台",
		Tags: []string{"platform.data", "search.entity"},
		Ops: stubOps(platform.SearchSpec{
			Scopes: []platform.SearchScope{platform.SearchEntity},
			Search: func(ctx context.Context, p platform.SearchParams) (platform.FetchPage, error) {
				calls++
				return platform.FetchPage{}, errors.New("上游 502")
			},
		}),
	}); err != nil {
		t.Fatal(err)
	}
	svc := newSvc(t, gw)
	p := QueryParams{Keyword: "抖动"}
	if _, err := svc.Query(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Query(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("失败结果不应缓存, calls = %d", calls)
	}
}

// TTL 过期与 FIFO 逐出。
func TestCacheTTLAndEviction(t *testing.T) {
	c := newCache()
	now := time.Now()
	ok := Result{Groups: []PlatformGroup{{Namespace: "a", Items: []platform.Item{}}}}
	c.store("k", ok, now)
	if _, hit := c.get("k", now); !hit {
		t.Error("TTL 内应命中")
	}
	if _, hit := c.get("k", now.Add(resultTTL)); hit {
		t.Error("到期后不应命中")
	}
	for i := 0; i < maxEntries+5; i++ {
		c.store(fmt.Sprintf("k%d", i), ok, now)
	}
	if len(c.entries) != maxEntries {
		t.Errorf("entries = %d, want %d", len(c.entries), maxEntries)
	}
	if _, hit := c.get("k0", now); hit {
		t.Error("最旧条目应被逐出")
	}
}
