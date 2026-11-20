// bili 数据平台：渠道/播放/弹幕能力。
// 经 platform.FeedOps 注册 channels / fetch / play / danmaku 四个 op。
// 扫码登录是纯 HTTP 协议（login.go 导出函数），不占 op 面；成功后写 sc.Creds。

package bili

import "mygo-host/platform"

// New 建 bili 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	c := newClient(sc)
	return platform.Spec{
		Namespace: "bili",
		Label:     "B站",
		Tags:      append([]string{"platform.data"}, platform.SearchTags(platform.SearchLive, platform.SearchEntity)...),
		Ops: platform.MergeOps(
			platform.FeedOps(platform.FeedSpec{
				Channels: channels(c),
				Playback: newPlayback(c),
			}),
			searchOps(c),
		),
	}
}
