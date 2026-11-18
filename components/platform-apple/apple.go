// apple 平台(Apple Podcasts):feed 协议平台,只有数据渠道(热门榜 / 关键词搜索),
// 无播放与弹幕能力。移植自 packages/platform-apple/src —— 渠道 key、参数表、
// JSON 形状与错误语义与 TS 侧一致;条目 kind=audio,url 为节目可订阅的 RSS feed。

package apple

import (
	"mygo-host/platform"
)

const (
	namespace = "apple"
	label     = "Apple Podcasts"
	tag       = "platform.data"
)

// New 建 apple 平台扩展(feed 协议:channels / fetch)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: namespace,
		Label:     label,
		Tags:      []string{tag},
		Ops:       platform.FeedOps(platform.FeedSpec{Channels: channels(sc)}),
	}
}
