// weibo 数据平台:仅聚合搜索 scope=entity(搜用户),数据必须由浏览器
// 被动采集——应用构造的 getIndex 请求实测返回 ok:-100,页面自发请求才是
// 唯一可用路径。匿名可用,不读 cookie、不做登录门禁。
// 旧 TS 渠道 weibo:user:search 的 kind=video 与 goTpl §4 的 channel 契约
// (entity 结果可选 liveStatus/roomId)无关;按 weibo.md §4.3 以规格为准,
// 只注册 search op,不声明订阅渠道与播放。

package weibo

import "mygo-host/platform"

const label = "微博"

// New 建 weibo 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "weibo",
		Label:     label,
		Tags:      append([]string{"platform.data"}, platform.SearchTags(platform.SearchEntity)...),
		Ops:       searchOps(sc),
	}
}
