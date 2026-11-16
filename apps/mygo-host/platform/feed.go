// feed 契约:平台把上游数据归一成的协议对象(与 platform-contracts/src/feed.ts 对齐)。
// TS 的 discriminated union 在 Go 侧收平为单个 Item 结构(Kind 判别 + 全可选字段指针),
// JSON 输出与 TS JSON.stringify 完全一致。

package platform

// Kind 条目类型判别值。
type Kind string

const (
	KindArticle  Kind = "article"
	KindSocial   Kind = "social"
	KindVideo    Kind = "video"
	KindAudio    Kind = "audio"
	KindLive     Kind = "live"
	KindCategory Kind = "category"
)

// Author 条目作者。
type Author struct {
	Name   string  `json:"name"`
	Avatar *string `json:"avatar,omitempty"`
	Handle *string `json:"handle,omitempty"`
}

// StreamingFormat 流形态。
type StreamingFormat string

const (
	FormatHLS        StreamingFormat = "hls"
	FormatDASH       StreamingFormat = "dash"
	FormatProgressve StreamingFormat = "progressive"
)

// Stream 可播流描述(直播多档位/视频 DASH 等)。
type Stream struct {
	URL          string            `json:"url"`
	Format       *string           `json:"format,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Quality      *string           `json:"quality,omitempty"`
	Rate         *int              `json:"rate,omitempty"`
	DashManifest *string           `json:"dashManifest,omitempty"`
}

// AttachmentKind 附件类型。
type AttachmentKind string

// Attachment 图文条目的媒体附件。
type Attachment struct {
	Kind            AttachmentKind   `json:"kind"`
	URL             string           `json:"url"`
	Title           *string          `json:"title,omitempty"`
	MimeType        *string          `json:"mimeType,omitempty"`
	Poster          *string          `json:"poster,omitempty"`
	Width           *int             `json:"width,omitempty"`
	Height          *int             `json:"height,omitempty"`
	AspectRatio     *float64         `json:"aspectRatio,omitempty"`
	DurationSec     *float64         `json:"durationSec,omitempty"`
	Bitrate         *int             `json:"bitrate,omitempty"`
	StreamingFormat *StreamingFormat `json:"streamingFormat,omitempty"`
	IsLiveNow       *bool            `json:"isLiveNow,omitempty"`
	Lang            *string          `json:"lang,omitempty"`
}

// SocialImage 社交图片(瀑布流;Live Photo 有短视频直链)。
type SocialImage struct {
	URL       string  `json:"url"`
	Width     *int    `json:"width,omitempty"`
	Height    *int    `json:"height,omitempty"`
	LiveVideo *string `json:"liveVideo,omitempty"`
}

// ItemChannel 视频条目的来源频道(YouTube 频道名等)。
type ItemChannel struct {
	Name   string  `json:"name"`
	Avatar *string `json:"avatar,omitempty"`
}

// LiveStatus 直播状态。
type LiveStatus string

const (
	LiveStatusLive    LiveStatus = "live"
	LiveStatusOffline LiveStatus = "offline"
	LiveStatusUnknown LiveStatus = "unknown"
)

// Item 平台条目(TS Item union 的扁平化;Kind 判别)。必需字段:id/sourceId/kind/title/fetchedAt。
type Item struct {
	ID          string  `json:"id"`
	SourceID    string  `json:"sourceId"`
	Kind        Kind    `json:"kind"`
	Title       string  `json:"title"`
	FetchedAt   int64   `json:"fetchedAt"`
	URL         *string `json:"url,omitempty"`
	Summary     *string `json:"summary,omitempty"`
	Thumbnail   *string `json:"thumbnail,omitempty"`
	Author      *Author `json:"author,omitempty"`
	PublishedAt *int64  `json:"publishedAt,omitempty"`
	Raw         RawJSON `json:"raw,omitempty"`

	MimeType    *string  `json:"mimeType,omitempty"`
	Poster      *string  `json:"poster,omitempty"`
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	AspectRatio *float64 `json:"aspectRatio,omitempty"`

	// Article / Social 共有
	Content       *string      `json:"content,omitempty"`
	ContentFormat *string      `json:"contentFormat,omitempty"`
	Media         []Attachment `json:"media,omitempty"`

	// Social
	Images    []SocialImage `json:"images,omitempty"`
	Likes     *int          `json:"likes,omitempty"`
	Reposts   *int          `json:"reposts,omitempty"`
	Replies   *int          `json:"replies,omitempty"`
	IsLiked   *bool         `json:"isLiked,omitempty"`
	Video     *string       `json:"video,omitempty"`
	HasVideo  *bool         `json:"hasVideo,omitempty"`
	XsecToken *string       `json:"xsecToken,omitempty"`

	// Video / Audio
	Duration    *float64     `json:"duration,omitempty"`
	MembersOnly *bool        `json:"membersOnly,omitempty"`
	Stream      *Stream      `json:"stream,omitempty"`
	Channel     *ItemChannel `json:"channel,omitempty"`
	Artist      *string      `json:"artist,omitempty"`
	Album       *string      `json:"album,omitempty"`

	// Video/Audio/Live 公共媒体字段
	DurationSec     *int             `json:"durationSec,omitempty"`
	Bitrate         *int             `json:"bitrate,omitempty"`
	StreamingFormat *StreamingFormat `json:"streamingFormat,omitempty"`
	IsLiveNow       *bool            `json:"isLiveNow,omitempty"`
	Lang            *string          `json:"lang,omitempty"`

	// Live(LivePlatformId 字符串:bilibili/douyu/huya/douyin/youtube/twitch)
	Platform     string      `json:"platform,omitempty"`
	RoomID       *string     `json:"roomId,omitempty"`
	LiveStatus   *LiveStatus `json:"liveStatus,omitempty"`
	Online       *int        `json:"online,omitempty"`
	IsRecord     *bool       `json:"isRecord,omitempty"`
	Introduction *string     `json:"introduction,omitempty"`
	ShowTime     *string     `json:"showTime,omitempty"`

	// Category
	CategoryChannel *string           `json:"categoryChannel,omitempty"`
	CategoryInfo    map[string]string `json:"categoryInfo,omitempty"`
}

// Str 拷贝字符串并取指针(可选字段赋值用)。
func Str(s string) *string { return &s }

// Int 拷贝数值并取指针(0 为合法值时必须走指针以保 JSON 语义)。
func Int(v int) *int { return &v }

// Int64 拷贝 int64 并取指针。
func Int64(v int64) *int64 { return &v }

// Float64 拷贝 float64 并取指针。
func Float64(v float64) *float64 { return &v }

// Bool 拷贝 bool 并取指针。
func Bool(v bool) *bool { return &v }

// StrOmit 空串 → nil(对应 TS `x || undefined`);非空取指针。
func StrOmit(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// IntOmit 0 → nil;非 0 取指针。
func IntOmit(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// Int64Omit 0 → nil;非 0 取指针。
func Int64Omit(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// Float64Omit 0 → nil;非 0 取指针。
func Float64Omit(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
