// twitch 平台:匿名 GraphQL 元数据渠道(直播订阅/热门/分区/分区列表/搜频道)
// + 直播 HLS 播放解析,经 feedOps 注册 channels / fetch / play(无弹幕实现)。
// 对齐 packages/platform-twitch/src;GraphQL Client-ID 与播放 token 查询见 client.go。

package twitch

import (
	"context"

	"mygo-host/platform"
)

const label = "Twitch"

// New 建 twitch 平台扩展。
func New(sc *platform.ServiceContext) platform.Spec {
	playback := &platform.Playback{
		Platform: "twitch",
		Routes:   twitchRoutes(),
		ResolveLivePlay: func(ctx context.Context, id string, _ platform.ResolveInfo) ([]platform.Stream, error) {
			return resolveTwitchLivePlay(ctx, sc, id)
		},
	}
	return platform.Spec{
		Namespace: "twitch",
		Label:     label,
		Tags:      []string{"platform.data"},
		Ops: platform.FeedOps(platform.FeedSpec{
			Channels: twitchChannels(sc),
			Playback: playback,
		}),
	}
}

// twitchChannels 渠道清单(顺序与 TS register.ts 一致)。
func twitchChannels(sc *platform.ServiceContext) []platform.DataChannel {
	return []platform.DataChannel{
		&liveChannel{sc: sc},
		&hotChannel{sc: sc},
		&categoryChannel{sc: sc},
		newCategoriesChannel(sc),
		newUserSearchChannel(sc),
	}
}
