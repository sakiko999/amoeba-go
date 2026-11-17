// twitch 平台:匿名 GraphQL 元数据渠道(直播订阅/回放/热门/分区/分区列表/搜频道)
// + 直播与回放 HLS 播放解析 + VOD 评论弹幕(直播弹幕 IRC 二期未实现),
// 经 feedOps 注册 channels / fetch / play / danmaku。
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
		ResolveVideoPlay: func(ctx context.Context, id string, _ platform.ResolveInfo) (*platform.VideoPlayResult, error) {
			return resolveTwitchVODPlay(ctx, sc, id)
		},
		GetDanmaku: func(id string, opts platform.DanmakuInfo) platform.DanmakuStream {
			// 与 douyu 同:显式 vod 走评论分页;默认 live(未实现,IRC 二期)。
			if opts.Kind == "vod" {
				return vodDanmakuStream(sc, id)
			}
			return unimplementedLiveDanmaku(sc)
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

// twitchChannels 渠道清单(顺序与 TS register.ts 一致;twitch:vod 为 Go 侧新增)。
func twitchChannels(sc *platform.ServiceContext) []platform.DataChannel {
	return []platform.DataChannel{
		&liveChannel{sc: sc},
		&vodChannel{sc: sc},
		&hotChannel{sc: sc},
		&categoryChannel{sc: sc},
		newCategoriesChannel(sc),
		newUserSearchChannel(sc),
	}
}
