// 虎牙(huya)直播平台:数据渠道 + 直播播放/弹幕能力,经 api.gateway 的 feed
// 协议暴露(channels / fetch / play / danmaku)。对齐 packages/platform-huya/src:
// 渠道装配见 channels*.go / discover.go,拉流解析 playback.go,弹幕与 Tars
// 编解码见 danmaku*.go / tars.go。

package huya

import "mygo-host/platform"

const (
	label = "虎牙"
	tag   = "platform.data"
)

// New 建 huya 平台扩展(HTTP/时钟由宿主注入;本平台不依赖 JS / cache / cookie)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "huya",
		Label:     label,
		Tags:      append([]string{tag}, platform.SearchTags(platform.SearchLive, platform.SearchChannel)...),
		Ops: platform.MergeOps(
			platform.FeedOps(platform.FeedSpec{Channels: channels(sc), Playback: playback(sc)}),
			searchOps(sc),
		),
	}
}
