// douyu 数据平台:对齐 packages/platform-douyu/src 的渠道/播放/弹幕能力,
// 经 platform.FeedOps 注册 channels / fetch / play / danmaku。
// 签名链维持 JS 原文执行(CRYPTO_JS + 服务端 crptext,见 cryptojs.go),
// 流走 getH5Play,弹幕走 STT 文本协议。

package douyu

import "mygo-host/platform"

const (
	label = "斗鱼"
	tag   = "platform.data"
)

// New 建 douyu 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "douyu",
		Label:     label,
		Tags:      append([]string{tag}, platform.SearchTags(platform.SearchLive, platform.SearchChannel)...),
		Ops: platform.MergeOps(
			platform.FeedOps(platform.FeedSpec{
				Channels: channels(sc),
				Playback: newPlayback(sc),
			}),
			searchOps(sc),
		),
	}
}
