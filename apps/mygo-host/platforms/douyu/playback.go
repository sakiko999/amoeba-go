// douyu 播放/弹幕能力 + URL 路由(对齐 TS playback.ts)。
// 直播房间 URL → roomId;play 走签名/getH5Play,弹幕走 STT WS。

package douyu

import (
	"context"
	"regexp"

	"mygo-host/platform"
)

var (
	routeTestRe = regexp.MustCompile(`(?:^|[./])douyu\.com/(\d+)`)
	routeIDRe   = regexp.MustCompile(`douyu\.com/(\d+)`)
)

// newPlayback 组装播放能力;sc 供拉流(签名/HTTP/日志)使用。
func newPlayback(sc *platform.ServiceContext) *platform.Playback {
	return &platform.Playback{
		Platform: "douyu",
		Routes: []platform.Route{{
			Re:   routeTestRe,
			Kind: "live",
			ExtractID: func(url string) (string, bool) {
				match := routeIDRe.FindStringSubmatch(url)
				if len(match) < 2 {
					return "", false
				}
				return match[1], true
			},
		}},
		ResolveLivePlay: func(ctx context.Context, id string, info platform.ResolveInfo) ([]platform.Stream, error) {
			return resolveStreams(ctx, sc, id)
		},
		GetDanmaku: func(id string, opts platform.DanmakuInfo) platform.DanmakuStream {
			return danmakuStream(sc, id)
		},
	}
}
