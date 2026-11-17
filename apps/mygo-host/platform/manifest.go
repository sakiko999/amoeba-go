// 内置服务组件的静态清单契约:字段与 TS 侧 component.json(kind=service)
// 同构(id/kind/name/version/provides/inject)。Go 服务编译期内置,清单文件
// 经 go:embed 进二进制(目录自描述,无运行时 IO);服务与 page 组件在组件
// 清单/依赖图平级展示。provides 的 expose 是元数据(进程内直连,方法即
// 接口,运行时不消费)。

package platform

import (
	"context"
	"encoding/json"
	"fmt"
)

// ProvideDecl 声明一个提供的服务名(两段 DNS label,如 api.gateway)。
type ProvideDecl struct {
	Name   string   `json:"name"`
	Expose []string `json:"expose,omitempty"`
}

// ServiceManifest 服务组件的清单声明(与 page 的 component.json 平级)。
// 文件形态时 kind 固定 "service"(防把页面清单装进服务管线)。
type ServiceManifest struct {
	ID       string        `json:"id"`
	Kind     string        `json:"kind,omitempty"`
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Provides []ProvideDecl `json:"provides,omitempty"`
	Inject   []string      `json:"inject,omitempty"`
}

// Registrar Activate 时可用的依赖集合。第一阶段只有网关注册面
// (*gateway.Gateway 天然满足);出现新依赖形态时再扩展。
type Registrar interface {
	Register(spec Spec) error
}

// OpCaller 是网关的调用面(*gateway.Gateway 天然满足,对应 provides.expose
// 里的 call):需要消费其他服务 op 的组件(player 调 <ns>.play)在 Activate
// 里对 Registrar 断言取用——注册面之外的第二种依赖形态。
type OpCaller interface {
	Call(ctx context.Context, op string, params json.RawMessage) (any, error)
}

// Service 内置服务组件:Manifest 供组件清单/拓扑排序,Activate 在全部
// inject 依赖就绪后按拓扑序调用。Deactivate(优雅停机)是独立的后续缺口,
// 需要时再扩展接口——内置清单集中在装配层一处,补全代价低。
type Service interface {
	Manifest() ServiceManifest
	Activate(r Registrar) error
}

// SpecService 把纯 Spec 工厂包装为服务组件(Activate = 经 Registrar 注册)。
func SpecService(m ServiceManifest, spec func() Spec) Service {
	return &specService{m: m, spec: spec}
}

type specService struct {
	m    ServiceManifest
	spec func() Spec
}

func (s *specService) Manifest() ServiceManifest { return s.m }

func (s *specService) Activate(r Registrar) error { return r.Register(s.spec()) }

// ParseServiceManifest 解析组件目录嵌入的 component.json(字段与 TS/page
// 组件同构;main 等多余字段忽略)。
func ParseServiceManifest(raw []byte) (ServiceManifest, error) {
	var m ServiceManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return ServiceManifest{}, fmt.Errorf("component.json 解析失败: %w", err)
	}
	if m.Kind != "service" {
		return ServiceManifest{}, fmt.Errorf("component.json kind 应为 service,got %q(id %s)", m.Kind, m.ID)
	}
	return m, nil
}

// FileService 从嵌入的 component.json 解析 manifest 并包装 Spec 工厂
// (目录形态与 page/TS 组件同构;文件经 go:embed 进二进制,无运行时 IO)。
func FileService(raw []byte, spec func() Spec) (Service, error) {
	m, err := ParseServiceManifest(raw)
	if err != nil {
		return nil, err
	}
	return SpecService(m, spec), nil
}
