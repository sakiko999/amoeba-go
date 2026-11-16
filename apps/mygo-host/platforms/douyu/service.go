// 平台包的服务组件声明:manifest 对齐 TS component.json(inject api.gateway),
// Activate = 经网关注册 Spec。内置服务由装配层按拓扑序激活。

package douyu

import "mygo-host/platform"

// NewService 平台服务组件(与 page 组件平级的清单成员)。
func NewService(sc *platform.ServiceContext) platform.Service {
	return platform.SpecService(
		platform.ServiceManifest{
			ID: "platform-douyu", Name: label, Version: "0.1.0",
			Inject: []string{"api.gateway"},
		},
		func() platform.Spec { return New(sc) },
	)
}
