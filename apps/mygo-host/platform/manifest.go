// 内置服务组件的静态清单契约:对齐 TS 侧 component.json(kind=service)的
// 字段(id/name/version/provides/inject)。Go 服务编译期内置,manifest 是
// 代码里的声明而非目录文件;服务与 page 组件在组件清单/依赖图平级展示。
// provides 的 expose 是元数据(进程内直连,方法即接口,运行时不消费)。

package platform

// ProvideDecl 声明一个提供的服务名(两段 DNS label,如 api.gateway)。
type ProvideDecl struct {
	Name   string   `json:"name"`
	Expose []string `json:"expose,omitempty"`
}

// ServiceManifest 服务组件的清单声明(与 page 的 component.json 平级)。
type ServiceManifest struct {
	ID       string        `json:"id"`
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

// Service 内置服务组件:Manifest 供组件清单/拓扑排序,Activate 在全部
// inject 依赖就绪后按拓扑序调用。Deactivate(优雅停机)是独立的后续缺口,
// 需要时再扩展接口——内置清单集中在装配层一处,补全代价低。
type Service interface {
	Manifest() ServiceManifest
	Activate(r Registrar) error
}

// SpecService 把纯 Spec 工厂包装为服务组件(Activate = 经 Registrar 注册)。
// 各平台包只需声明 manifest,不重复实现接口。
func SpecService(m ServiceManifest, spec func() Spec) Service {
	return &specService{m: m, spec: spec}
}

type specService struct {
	m    ServiceManifest
	spec func() Spec
}

func (s *specService) Manifest() ServiceManifest { return s.m }

func (s *specService) Activate(r Registrar) error { return r.Register(s.spec()) }
