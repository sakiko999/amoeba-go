// youtube 平台(feed 协议):7 个数据渠道 + 播放/弹幕能力注册进 api.gateway。
// 渠道声明与 TS register.ts 完全一致;播放按 URL 路由(watch?v= / youtu.be),
// 弹幕是 live chat 轮询。Go 版对齐 packages/platform-youtube/src 全部能力。

package youtube

import (
	"context"
	"net/url"
	"regexp"

	"mygo-host/platform"
)

var (
	watchRouteRe   = regexp.MustCompile(`(?:^|[./])youtube\.com/watch\?v=([^&]+)`)
	shortRouteRe   = regexp.MustCompile(`(?:^|[./])youtu\.be/([^?/]+)`)
	shortExtractRe = regexp.MustCompile(`youtu\.be/([^?/]+)`)
)

// routes 直播与点播 URL 同形,统一按 video 路由(直播由 resolveVideoPlay 内部判 HLS)。
func routes() []platform.Route {
	return []platform.Route{
		{
			Re:   watchRouteRe,
			Kind: "video",
			ExtractID: func(raw string) (string, bool) {
				parsed, err := url.Parse(raw)
				if err != nil || parsed.Scheme == "" || parsed.Host == "" {
					return "", false
				}
				videoID := parsed.Query().Get("v")
				if videoID == "" {
					return "", false
				}
				return videoID, true
			},
		},
		{
			Re:   shortRouteRe,
			Kind: "video",
			ExtractID: func(raw string) (string, bool) {
				match := shortExtractRe.FindStringSubmatch(raw)
				if len(match) < 2 {
					return "", false
				}
				return match[1], true
			},
		},
	}
}

func playback(sc *platform.ServiceContext) *platform.Playback {
	return &platform.Playback{
		Platform: "youtube",
		Routes:   routes(),
		ResolveVideoPlay: func(ctx context.Context, id string, info platform.ResolveInfo) (*platform.VideoPlayResult, error) {
			streams, err := resolveYoutubeStreams(ctx, sc, id)
			if err != nil {
				return nil, err
			}
			return &platform.VideoPlayResult{Streams: streams}, nil
		},
		// YouTube 弹幕是 live chat 轮询(videoId);VOD 无弹幕。
		GetDanmaku: func(id string, opts platform.DanmakuInfo) platform.DanmakuStream {
			return createLiveChatPoller(sc, id)
		},
	}
}

// New 建 youtube 平台扩展(与 services 层接线签名固定)。
func New(sc *platform.ServiceContext) platform.Spec {
	return platform.Spec{
		Namespace: "youtube",
		Label:     "YouTube",
		Tags:      []string{"platform.data"},
		Ops: platform.FeedOps(platform.FeedSpec{
			Channels: channels(sc),
			Playback: playback(sc),
		}),
	}
}
