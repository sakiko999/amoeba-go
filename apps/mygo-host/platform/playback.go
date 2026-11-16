// 平台播放/弹幕契约(对齐 platform-kit/runtime/playback.ts):
// URL → id/kind 的路由知识由各平台自带(gateway 模型下无全局路由表)。

package platform

import (
	"context"
	"regexp"
)

// ResolveInfo resolver 解析参数:复用 SourceInfo,必要时带 cookie。
type ResolveInfo map[string]string

// Cookie 取解析参数里的显式 cookie(无则空串)。
func (r ResolveInfo) Cookie() string { return r["cookie"] }

// PlaybackChapter 章节时间点(秒 + 标题;目前只有 bili 有)。
type PlaybackChapter struct {
	T     float64 `json:"t"`
	Label string  `json:"label"`
}

// Route 平台自带 URL 路由:URL 形态 → 条目 id + 播放种类。
type Route struct {
	// Re 匹配 item.url 的形态(正则 test)。
	Re *regexp.Regexp
	// Kind "video" | "live"。
	Kind string
	// ExtractID 从 url 提取条目 id;false 视为不命中。
	ExtractID func(url string) (string, bool)
}

// MatchRoute 按 url 在路由表里找 {id, kind};未命中 ok=false。
func MatchRoute(routes []Route, url string) (id, kind string, ok bool) {
	for _, route := range routes {
		if route.Re == nil || !route.Re.MatchString(url) {
			continue
		}
		extracted, found := route.ExtractID(url)
		if found && extracted != "" {
			return extracted, route.Kind, true
		}
	}
	return "", "", false
}

// VideoPlayResult 视频可播流解析结果。
type VideoPlayResult struct {
	Streams  []Stream          `json:"streams"`
	Chapters []PlaybackChapter `json:"chapters,omitempty"`
}

// Playback 平台播放/弹幕能力;未实现的能力字段为 nil(与 TS 可选方法同语义)。
type Playback struct {
	// Platform 平台自述 id(日志/诊断用,如 "bilibili" / "huya")。
	Platform string
	// Routes 本平台 URL 路由(play/danmaku 的 url 参数据此取 id/kind)。
	Routes []Route
	// ResolveVideoPlay 视频可播流解析。
	ResolveVideoPlay func(ctx context.Context, id string, info ResolveInfo) (*VideoPlayResult, error)
	// ResolveLivePlay 直播可播流解析。
	ResolveLivePlay func(ctx context.Context, id string, info ResolveInfo) ([]Stream, error)
	// GetDanmaku 弹幕流(无弹幕能力的平台为 nil)。
	GetDanmaku func(id string, opts DanmakuInfo) DanmakuStream
}
