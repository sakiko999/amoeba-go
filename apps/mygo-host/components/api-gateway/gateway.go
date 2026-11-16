// api-gateway —— 统一平台 API 注册中心(api.gateway)的 Go 版。
// 平台以「命名空间.操作」注册(如 tmdb.search / bili.channels);消费方经
// Call/List 使用。只有注册表与分发,不含任何域逻辑。
// 进程内函数直连取代 TS 侧 provide/inject 服务对象;Spec → Ops 的解析
// (宿主能力注入)由 services 层完成,网关只收已构建的操作表。

package gateway

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"mygo-host/platform"
)

// OpDesc op 内省描述。
type OpDesc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ApiInfo 一个平台的注册信息(List 返回元素)。
type ApiInfo struct {
	Namespace string   `json:"namespace"`
	Label     string   `json:"label"`
	Tags      []string `json:"tags,omitempty"`
	Ops       []OpDesc `json:"ops"`
}

// Gateway 平台 API 注册中心(并发安全);NewService 把它包装为根服务
// 组件(provides api.gateway,manifest 见本目录 component.json)。
type Gateway struct {
	mu   sync.RWMutex
	apis map[string]platform.Ops
	meta map[string]ApiInfo
	logf func(format string, args ...any)
}

var (
	namespaceRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	opRe        = regexp.MustCompile(`^[a-z][a-zA-Z0-9-]*$`)
)

// New 建网关。自环自检面(gateway.ping)随构造注册(绕过 Register 的保留字
// 校验,与 TS 侧直接 apis.set 同语义);自检执行移入 Activate(根服务激活)。
func New(logf func(format string, args ...any)) *Gateway {
	g := &Gateway{apis: map[string]platform.Ops{}, meta: map[string]ApiInfo{}, logf: logf}
	g.apis["gateway"] = platform.GuardOps("gateway", platform.Ops{
		"ping": {
			Description: "回显参数,验证注册与分发链路",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				var echoed any
				if err := json.Unmarshal(params, &echoed); err != nil {
					echoed = nil
				}
				return map[string]any{"pong": true, "params": echoed}, nil
			},
		},
	})
	g.meta["gateway"] = ApiInfo{Namespace: "gateway", Label: "网关自检", Ops: []OpDesc{{Name: "ping", Description: "回显参数,验证注册与分发链路"}}}
	return g
}

// Manifest 根服务声明(对齐 TS api-gateway component.json)。
//
//go:embed component.json
var manifestRaw []byte

// NewService 把网关包装为根服务组件:manifest 来自本目录 component.json
// (与 page/平台组件目录同构);Activate = 注册/分发链路自检。
func NewService(g *Gateway) (platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, err
	}
	return &rootService{g: g, m: m}, nil
}

type rootService struct {
	g *Gateway
	m platform.ServiceManifest
}

func (s *rootService) Manifest() platform.ServiceManifest { return s.m }

// Activate 根服务自检:注册/分发链路可用才算激活成功(无依赖,忽略 r)。
func (s *rootService) Activate(platform.Registrar) error {
	if _, err := s.g.Call(context.Background(), "gateway.ping", json.RawMessage(`{"check":1}`)); err != nil {
		return fmt.Errorf("gateway self-check FAILED: %w", err)
	}
	if s.g.logf != nil {
		s.g.logf("self-check ok")
	}
	return nil
}

// Register 注册一个平台扩展(ops 会被 GuardOps 统一包装 PlatformOpError)。
// 同 namespace 重复注册整体覆盖。
func (g *Gateway) Register(spec platform.Spec) error {
	if !namespaceRe.MatchString(spec.Namespace) {
		return fmt.Errorf("api.gateway: 非法命名空间 %q(平台 id,单段小写,如 tmdb / bili)", spec.Namespace)
	}
	if spec.Namespace == "gateway" {
		return fmt.Errorf("api.gateway: 命名空间 %q 为网关保留字", spec.Namespace)
	}
	ops := spec.Ops
	if len(ops) == 0 {
		return fmt.Errorf("api.gateway: %s 未提供 ops", spec.Namespace)
	}
	info := ApiInfo{Namespace: spec.Namespace, Label: spec.Label, Tags: spec.Tags}
	for name, op := range ops {
		if !opRe.MatchString(name) {
			return fmt.Errorf("api.gateway: %s 非法操作名 %q", spec.Namespace, name)
		}
		if op.Invoke == nil {
			return fmt.Errorf("api.gateway: %s.%s 缺少 invoke", spec.Namespace, name)
		}
		info.Ops = append(info.Ops, OpDesc{Name: name, Description: op.Description})
	}
	sort.Slice(info.Ops, func(i, j int) bool { return info.Ops[i].Name < info.Ops[j].Name })

	g.mu.Lock()
	defer g.mu.Unlock()
	g.apis[spec.Namespace] = platform.GuardOps(spec.Namespace, ops)
	g.meta[spec.Namespace] = info
	if g.logf != nil {
		g.logf("api registered: %s (%d ops)", spec.Namespace, len(ops))
	}
	return nil
}

// Call 分发「平台.操作」;params 为 nil 时以空对象语义传给平台。
func (g *Gateway) Call(ctx context.Context, op string, params json.RawMessage) (any, error) {
	dot := strings.IndexByte(op, '.')
	if dot <= 0 || dot == len(op)-1 {
		return nil, fmt.Errorf("api.gateway: 非法操作名 %q(应为 平台.操作,如 tmdb.search)", op)
	}
	namespace, name := op[:dot], op[dot+1:]
	g.mu.RLock()
	ops, ok := g.apis[namespace]
	g.mu.RUnlock()
	if !ok {
		return nil, g.unknownOp(op)
	}
	handler, ok := ops[name]
	if !ok {
		return nil, g.unknownOp(op)
	}
	if params == nil {
		params = json.RawMessage("{}")
	}
	return handler.Invoke(ctx, params)
}

// List 结构化清单(按 namespace 排序)。
func (g *Gateway) List() []ApiInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	infos := make([]ApiInfo, 0, len(g.meta))
	for _, info := range g.meta {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Namespace < infos[j].Namespace })
	return infos
}

func (g *Gateway) unknownOp(op string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ns := make([]string, 0, len(g.apis))
	for k := range g.apis {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	return fmt.Errorf("api.gateway: 未注册操作 %q(已注册平台:%s)", op, strings.Join(ns, ", "))
}
