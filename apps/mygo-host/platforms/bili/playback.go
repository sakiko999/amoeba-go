// bili 播放/弹幕能力 + URL 路由（注册进 api.gateway）：
//   - 视频：bvid/av → cid → DASH/durl 全档位 + 章节；
//   - 直播：roomId → getRoomPlayInfo 全档位；
//   - 弹幕：kind 缺省走 VOD（与 TS getDanmaku 的 `opts.kind ?? "vod"` 一致）。
// TS 路由正则用非捕获组，Go RE2 不支持，这里改为捕获组（MatchRoute 只做 MatchString）。

package bili

import (
	"context"
	"regexp"

	"mygo-host/platform"
)

var (
	videoBvRouteRe    = regexp.MustCompile(`(^|[./])bilibili\.com/video/(BV\w+)`)
	videoAvRouteRe    = regexp.MustCompile(`(?i)(^|[./])bilibili\.com/video/(av\d+)`)
	liveRoomRouteRe   = regexp.MustCompile(`(^|[./])live\.bilibili\.com/(\d+)`)
	videoBvExtractRe  = regexp.MustCompile(`/video/(BV\w+)`)
	videoAvExtractRe  = regexp.MustCompile(`(?i)/(av\d+)`)
	liveRoomExtractRe = regexp.MustCompile(`live\.bilibili\.com/(\d+)`)
)

func newPlayback(c *client) *platform.Playback {
	return &platform.Playback{
		Platform: "bilibili",
		Routes: []platform.Route{
			{Re: videoBvRouteRe, Kind: "video", ExtractID: extractByRe(videoBvExtractRe)},
			{Re: videoAvRouteRe, Kind: "video", ExtractID: extractByRe(videoAvExtractRe)},
			{Re: liveRoomRouteRe, Kind: "live", ExtractID: extractByRe(liveRoomExtractRe)},
		},
		ResolveVideoPlay: func(ctx context.Context, id string, info platform.ResolveInfo) (*platform.VideoPlayResult, error) {
			return c.resolveVideoPlay(ctx, id, info)
		},
		ResolveLivePlay: func(ctx context.Context, id string, info platform.ResolveInfo) ([]platform.Stream, error) {
			return c.resolveLivePlay(ctx, id, info)
		},
		GetDanmaku: c.getDanmaku,
	}
}

func extractByRe(re *regexp.Regexp) func(string) (string, bool) {
	return func(url string) (string, bool) {
		m := re.FindStringSubmatch(url)
		if len(m) != 2 {
			return "", false
		}
		return m[1], true
	}
}

// getDanmaku kind 缺省 = vod(TS: opts?.kind === "vod" ? vod : live,playback 层已填缺省)。
func (c *client) getDanmaku(id string, opts platform.DanmakuInfo) platform.DanmakuStream {
	kind := opts.Kind
	if kind == "" {
		kind = "vod"
	}
	if kind == "vod" {
		return c.vodDanmakuStream(id, opts.Cookie)
	}
	return c.liveDanmakuStream(id, opts.Cookie)
}
