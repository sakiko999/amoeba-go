// jsrunner 的服务组件包装:manifest 来自本目录 component.json,登记进
// 组件清单(与平台/page 平级)。Activate 无网关动作——host.js 是宿主能力,
// 经 ServiceContext.JS 供给各平台(不暴露为网关 op,页面侧不可直调);
// 窗口 Init 由宿主在 app ready 后调用(见 webview.go)。

package jsrunner

import (
	_ "embed"

	"mygo-host/platform"
)

//go:embed component.json
var manifestRaw []byte

// NewService 把 runner 包装为服务组件(仅清单登记,Activate 为 no-op)。
func NewService(r *WebView) (platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, err
	}
	return &jsService{r: r, m: m}, nil
}

type jsService struct {
	r *WebView
	m platform.ServiceManifest
}

func (s *jsService) Manifest() platform.ServiceManifest { return s.m }

func (s *jsService) Activate(platform.Registrar) error { return nil }
