// douyin 数据平台:搜索(scope=entity:抖音号/主页链接查号 + 关键词捕获)
// 与直播间取流(enter API + ABOGUS JS 签名)。登录态由「登录态与密钥」页扫码
// 写入载具 profile;关键词搜索仍以 sessionid 门禁判断是否已登录。

package douyin

import "mygo-host/platform"

const (
	label = "抖音"
	tag   = "platform.data"
)

// New 建 douyin 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "douyin",
		Label:     label,
		Tags:      append([]string{tag}, platform.SearchTags(platform.SearchEntity)...),
		Ops: platform.MergeOps(
			platform.FeedOps(platform.FeedSpec{Playback: newPlayback(sc)}),
			searchOps(sc),
		),
	}
}
