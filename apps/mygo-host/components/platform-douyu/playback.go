// douyu 播放/弹幕能力 + URL 路由(直播对齐 TS playback.ts;回放为 Go 侧新增,
// 依据 docs/douyu-replay-research.md)。直播房间 URL → roomId(kind=live);
// v.douyu.com/show|vmobile/<hash> → 回放(kind=video,弹幕走 VOD 分页接口)。

package douyu

import (
	"context"
	"regexp"

	"mygo-host/platform"
)

var (
	routeTestRe  = regexp.MustCompile(`(?:^|[./])douyu\.com/(\d+)`)
	routeIDRe    = regexp.MustCompile(`douyu\.com/(\d+)`)
	vodRouteRe   = regexp.MustCompile(`v\.douyu\.com/(?:show|vmobile)/([0-9A-Za-z]+)`)
	vodHashIDSet = regexp.MustCompile(`^[0-9A-Za-z]+$`)
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
		}, {
			Re:   vodRouteRe,
			Kind: "video",
			ExtractID: func(url string) (string, bool) {
				match := vodRouteRe.FindStringSubmatch(url)
				if len(match) < 2 || !vodHashIDSet.MatchString(match[1]) {
					return "", false
				}
				return match[1], true
			},
		}},
		ResolveLivePlay: func(ctx context.Context, id string, info platform.ResolveInfo) ([]platform.Stream, error) {
			return resolveStreams(ctx, sc, id)
		},
		ResolveVideoPlay: func(ctx context.Context, id string, info platform.ResolveInfo) (*platform.VideoPlayResult, error) {
			streams, err := resolveVideoStreams(ctx, sc, id)
			if err != nil {
				return nil, err
			}
			return &platform.VideoPlayResult{Streams: streams}, nil
		},
		GetDanmaku: func(id string, opts platform.DanmakuInfo) platform.DanmakuStream {
			// 默认 live(douyu 现行为);URL 路由命中 show/ 时 feedops 会填 vod。
			if opts.Kind == "vod" {
				return vodDanmakuStream(sc, id)
			}
			return danmakuStream(sc, id)
		},
	}
}
