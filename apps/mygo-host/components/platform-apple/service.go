// 平台服务组件:manifest 来自本目录 component.json(与 page/TS 组件同构,
// 经 go:embed 进二进制),Activate = 经网关注册 Spec。

package apple

import (
	_ "embed"

	"mygo-host/platform"
)

//go:embed component.json
var manifestRaw []byte

// NewService 平台服务组件(与 page 组件平级的清单成员)。
func NewService(sc *platform.ServiceContext) (platform.Service, error) {
	return platform.FileService(manifestRaw, func() platform.Spec { return New(sc) })
}
