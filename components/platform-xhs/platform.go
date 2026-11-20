// xhs 数据平台:仅聚合搜索 scope=entity(搜用户)。关键词搜索必须走浏览器:
// 搜索接口需要页面签名,无浏览器时没有可用降级路径(签名库不在本分支)。
// 扫码登录见 login.go(login.Flow);搜索未登录仍报 ErrBrowserLoginRequired。
// 只注册 search op,不声明订阅渠道与播放。

package xhs

import "mygo-host/platform"

const (
	label = "小红书"
	tag   = "platform.data"
)

// New 建 xhs 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "xhs",
		Label:     label,
		Tags:      append([]string{tag}, platform.SearchTags(platform.SearchEntity)...),
		Ops:       searchOps(sc),
	}
}
