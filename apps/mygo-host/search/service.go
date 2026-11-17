// Search 宿主服务:聚合搜索的平台发现与并发 fan-out。页面经
// window.mygo.call("Search.Sources"/"Search.Query", ...) 消费;每平台一个
// goroutine + 8s 超时,完成一个经 *mygo.Channel 推一个,全部完成后返回
// 按 Sources 顺序的权威快照。单平台失败是分组数据(error 字段),不阻塞
// 其他平台。方案:docs/mygo-search-page-plan.md §3.2。

package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"mygo-host/components/api-gateway"
	"mygo-host/platform"
)

// perPlatformTimeout 单平台搜索超时(var:测试可调小)。
var perPlatformTimeout = 8 * time.Second

// searchPageSize 阶段 1 只取首屏 20 条。
const searchPageSize = 20

type Service struct {
	gw *gateway.Gateway
}

func NewService(gw *gateway.Gateway) *Service { return &Service{gw: gw} }

type Source struct {
	Namespace string `json:"namespace"`
	Label     string `json:"label,omitempty"`
}

type QueryParams struct {
	Keyword   string               `json:"keyword"`
	Scope     platform.SearchScope `json:"scope"`
	Platforms []string             `json:"platforms,omitempty"` // 空 = 全部
}

type GroupError struct {
	Op      string `json:"op,omitempty"` // 如 "bili.search"
	Code    string `json:"code"`         // timeout | canceled | platform | internal
	Message string `json:"message"`
}

type PlatformGroup struct {
	Namespace string          `json:"namespace"`
	Label     string          `json:"label,omitempty"`
	Items     []platform.Item `json:"items"`
	Total     *int            `json:"total,omitempty"`
	HasMore   *bool           `json:"hasMore,omitempty"`
	Cursor    *string         `json:"cursor,omitempty"`
	Error     *GroupError     `json:"error,omitempty"`
	ElapsedMS int64           `json:"elapsedMs"`
}

type Result struct {
	Keyword   string               `json:"keyword"`
	Scope     platform.SearchScope `json:"scope"`
	Groups    []PlatformGroup      `json:"groups"` // 按 Sources 顺序,永远非 nil
	ElapsedMS int64                `json:"elapsedMs"`
}

// Sources 某 scope 可用的搜索平台(网关发现:platform.data + search.<scope> + search op)。
func (s *Service) Sources(_ context.Context, scope platform.SearchScope) ([]Source, error) {
	tag := "search." + string(scope)
	out := []Source{}
	for _, info := range s.gw.List() {
		if !contains(info.Tags, "platform.data") || !contains(info.Tags, tag) {
			continue
		}
		if !declared(info.Ops, "search") {
			continue
		}
		out = append(out, Source{Namespace: info.Namespace, Label: info.Label})
	}
	return out, nil
}

// Query 校验参数后并发 fan-out;groups 为 nil 时非流式(只返回快照)。
// 顶层 error 仅在参数非法时出现;页面关 channel → Send 失败 → 取消剩余平台。
func (s *Service) Query(ctx context.Context, p QueryParams, groups *mygo.Channel[PlatformGroup]) (Result, error) {
	p.Keyword = strings.TrimSpace(p.Keyword)
	if p.Keyword == "" {
		return Result{}, errors.New("search 需要 keyword 参数")
	}
	if p.Scope != platform.SearchLive && p.Scope != platform.SearchChannel {
		return Result{}, fmt.Errorf("search 不支持 scope %q(live/channel)", p.Scope)
	}
	var emit func(PlatformGroup) error
	if groups != nil {
		emit = groups.Send
	}
	sources := filterSources(s.sourcesOf(p.Scope), p.Platforms)
	return s.run(ctx, p, sources, emit), nil
}

func (s *Service) sourcesOf(scope platform.SearchScope) []Source {
	out, _ := s.Sources(context.Background(), scope)
	return out
}

func filterSources(sources []Source, only []string) []Source {
	if len(only) == 0 {
		return sources
	}
	want := map[string]bool{}
	for _, ns := range only {
		want[strings.TrimSpace(ns)] = true
	}
	out := make([]Source, 0, len(sources))
	for _, src := range sources {
		if want[src.Namespace] {
			out = append(out, src)
		}
	}
	return out
}

// run 并发查询全部平台;emit 按完成顺序推送(nil = 不流式)。
func (s *Service) run(ctx context.Context, p QueryParams, sources []Source, emit func(PlatformGroup) error) Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := time.Now()
	results := make([]PlatformGroup, len(sources))
	for i, src := range sources {
		results[i] = PlatformGroup{Namespace: src.Namespace, Label: src.Label, Items: []platform.Item{}}
	}
	type done struct {
		idx   int
		group PlatformGroup
	}
	queue := make(chan done)
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(idx int, src Source) {
			defer wg.Done()
			queue <- done{idx, s.queryOne(ctx, p, src)}
		}(i, src)
	}
	go func() {
		wg.Wait()
		close(queue)
	}()
	for item := range queue {
		results[item.idx] = item.group
		if emit != nil && emit(item.group) != nil {
			emit = nil // 页面停听:取消剩余平台并不再推送
			cancel()
		}
	}
	return Result{
		Keyword: p.Keyword, Scope: p.Scope, Groups: results,
		ElapsedMS: time.Since(started).Milliseconds(),
	}
}

// queryOne 查一个平台(超时隔离);任何失败进 group.error。
func (s *Service) queryOne(ctx context.Context, p QueryParams, src Source) PlatformGroup {
	ctx, cancel := context.WithTimeout(ctx, perPlatformTimeout)
	defer cancel()
	started := time.Now()
	group := PlatformGroup{Namespace: src.Namespace, Label: src.Label, Items: []platform.Item{}}
	params, err := json.Marshal(platform.SearchParams{Keyword: p.Keyword, Scope: p.Scope, Page: 1, PageSize: searchPageSize})
	if err == nil {
		var raw any
		raw, err = s.gw.Call(ctx, src.Namespace+".search", params)
		if err == nil {
			if page, ok := raw.(platform.FetchPage); ok {
				group.Items, group.Total, group.HasMore, group.Cursor = page.Items, page.Total, page.HasMore, page.Cursor
				if group.Items == nil {
					group.Items = []platform.Item{}
				}
			} else {
				err = fmt.Errorf("search 返回类型异常(%T)", raw)
			}
		}
	}
	if err != nil {
		group.Error = classify(err, src.Namespace+".search")
	}
	group.ElapsedMS = time.Since(started).Milliseconds()
	return group
}

func classify(err error, op string) *GroupError {
	code := "internal"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.Is(err, context.Canceled):
		code = "canceled"
	case errors.As(err, new(*platform.PlatformOpError)):
		code = "platform"
	}
	return &GroupError{Op: op, Code: code, Message: err.Error()}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func declared(ops []gateway.OpDesc, name string) bool {
	for _, op := range ops {
		if op.Name == name {
			return true
		}
	}
	return false
}
