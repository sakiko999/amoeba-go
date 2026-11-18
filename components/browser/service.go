// browser 载具的服务组件面:provides browser.proxy,消费者经网关
// browser.endpoint 取独立 WebView2 进程的 CDP 端点(见 carrier.go)。
// 子进程由同一二进制以 --browser-carrier 启动,与宿主共享代码但不共享
// WebView2 环境/端口/profile。

package browser

import (
	"context"
	_ "embed"
	"encoding/json"

	"mygo-host/platform"
)

//go:embed component.json
var manifestRaw []byte

// NewService 返回可供测试/宿主直调的服务与参与拓扑注册的组件包装。
func NewService(profileDir string, logf func(string, ...any)) (*Service, platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, nil, err
	}
	s := newService(profileDir, logf)
	return s, &component{s: s, m: m}, nil
}

type component struct {
	s *Service
	m platform.ServiceManifest
}

func (c *component) Manifest() platform.ServiceManifest { return c.m }

func (c *component) Activate(r platform.Registrar) error {
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
