// douyin 播放能力:live.douyin.com/<web_rid> → 直播间取流(kind=live)。
// 路由正则只用捕获组(RE2 兼容);extractId 取 pathname 最后一个非空段。

package douyin

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"mygo-host/platform"
)

var liveRouteRe = regexp.MustCompile(`(^|[./])live\.douyin\.com/\S+`)

// newPlayback 组装 douyin 播放能力;sc 供 enter 取流使用。
func newPlayback(sc *platform.ServiceContext) *platform.Playback {
	return &platform.Playback{
		Platform: "douyin",
		Routes: []platform.Route{{
			Re:        liveRouteRe,
			Kind:      "live",
			ExtractID: extractLiveID,
		}},
		ResolveLivePlay: func(ctx context.Context, id string, _ platform.ResolveInfo) ([]platform.Stream, error) {
			return resolveLiveStreams(ctx, sc, id)
		},
	}
}

// extractLiveID 对齐 TS extractId:绝对 URL 的 pathname 末段(不解码),空段或 "-" 不命中。
func extractLiveID(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	segments := strings.Split(path, "/")
	last := segments[len(segments)-1]
	if last == "" || last == "-" {
		return "", false
	}
	return last, true
}
