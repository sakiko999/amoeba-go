// browser 载具的服务组件面。生产路径经 NewPoolService 提供按平台隔离的
// 实例池(见 pool.go);NewService 保留单实例构造,供测试/探针直接驱动
// 一个载具进程(同一二进制以 --browser-carrier 启动)。

package browser

import (
	"context"
	_ "embed"
	"encoding/json"

	"mygo-host/platform"
)

//go:embed component.json
var manifestRaw []byte

// NewService 返回单个载具实例与参与拓扑注册的组件包装(测试/探针用)。
func NewService(profileDir string, logf func(string, ...any)) (*Service, platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, nil, err
	}
	s := newService(profileDir, logf)
	return s, &component{s: s, m: m}, nil
}

// NewPoolService 返回实例池与参与拓扑注册的组件包装(宿主生产路径)。
func NewPoolService(profileRoot string, logf func(string, ...any)) (*Pool, platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, nil, err
	}
	p := NewPool(profileRoot, logf)
	return p, &component{p: p, m: m}, nil
}

type component struct {
	s *Service
	p *Pool
	m platform.ServiceManifest
}

func (c *component) Manifest() platform.ServiceManifest { return c.m }

func (c *component) Activate(r platform.Registrar) error {
	if c.p != nil {
		return c.activatePool(r)
	}
	return r.Register(platform.Spec{
		Namespace: "browser",
		Label:     "爬虫浏览器载具",
		Tags:      []string{"browser.cdp"},
		Ops: platform.Ops{
			"endpoint": {
				Description: "启动(如未运行)并返回 CDP 端点 {port,http,ws}",
				Invoke: func(ctx context.Context, _ json.RawMessage) (any, error) {
					return c.s.Endpoint(ctx)
				},
			},
			"targetCount": {
				Description: "载具当前 page target 数(含载具自带的 about:blank 基础页)",
				Invoke: func(ctx context.Context, _ json.RawMessage) (any, error) {
					return c.s.TargetCount(ctx)
				},
			},
			"status": {
				Description: "载具进程状态 {running,pid,port,userDataDir,profileDir,detail}",
				Invoke: func(_ context.Context, _ json.RawMessage) (any, error) {
					return c.s.Status(), nil
				},
			},
			"reset": {
				Description: "停止子进程并删除 profile(登录态/缓存全部清除)",
				Invoke: func(ctx context.Context, _ json.RawMessage) (any, error) {
					return nil, c.s.Reset(ctx)
				},
			},
			"stop": {
				Description: "停止载具子进程(profile 保留)",
				Invoke: func(_ context.Context, _ json.RawMessage) (any, error) {
					return nil, c.s.Stop()
				},
			},
		},
	})
}

func (c *component) activatePool(r platform.Registrar) error {
	return r.Register(platform.Spec{
		Namespace: "browser",
		Label:     "爬虫浏览器载具",
		Tags:      []string{"browser.cdp"},
		Ops: platform.Ops{
			"status": {
				Description: "载具实例池状态 {running,instances:[{profile,pid,port,busy,warm}]},不启动进程",
				Invoke: func(_ context.Context, _ json.RawMessage) (any, error) {
					return c.p.Status(), nil
				},
			},
			"reset": {
				Description: "停止全部实例并删除 profile 根目录(登录态/缓存全部清除)",
				Invoke: func(ctx context.Context, _ json.RawMessage) (any, error) {
					return nil, c.p.ResetAll(ctx)
				},
			},
			"stop": {
				Description: "停止全部载具实例(profile 保留)",
				Invoke: func(_ context.Context, _ json.RawMessage) (any, error) {
					c.p.StopAll()
					return nil, nil
				},
			},
		},
	})
}
