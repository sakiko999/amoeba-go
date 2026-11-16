// 上游响应结构与内部数据形态。playerResponse 字段全可选(指针),映射层兜底;
// videoDetails 的 title/viewCount/lengthSeconds 用 any 承接(YouTube 可能给字符串或数字,
// 对应 TS 里的 String(...) 归一)。频道/直播解析的中间结构一并集中在此。

package youtube

type playerResponse struct {
	VideoDetails      *videoDetails      `json:"videoDetails"`
	PlayabilityStatus *playabilityStatus `json:"playabilityStatus"`
	StreamingData     *streamingData     `json:"streamingData"`
}

type videoDetails struct {
	Title         any   `json:"title"`
	Author        any   `json:"author"`
	ViewCount     any   `json:"viewCount"`
	IsLiveContent *bool `json:"isLiveContent"`
	IsLive        *bool `json:"isLive"`
	LengthSeconds any   `json:"lengthSeconds"`
}

type playabilityStatus struct {
	Status            *string        `json:"status"`
	Reason            *string        `json:"reason"`
	ErrorScreen       map[string]any `json:"errorScreen"`
	LiveStreamability any            `json:"liveStreamability"`
}

type streamingData struct {
	Formats         []playerFormat `json:"formats"`
	AdaptiveFormats []playerFormat `json:"adaptiveFormats"`
	HLSManifestURL  *string        `json:"hlsManifestUrl"`
	DASHManifestURL *string        `json:"dashManifestUrl"`
}

type playerFormat struct {
	Itag            *float64   `json:"itag"`
	URL             *string    `json:"url"`
	BaseURL         *string    `json:"baseUrl"`
	Cipher          *string    `json:"cipher"`
	SignatureCipher *string    `json:"signatureCipher"`
	MimeType        *string    `json:"mimeType"`
	Bitrate         *float64   `json:"bitrate"`
	Width           *float64   `json:"width"`
	Height          *float64   `json:"height"`
	FPS             *float64   `json:"fps"`
	QualityLabel    *string    `json:"qualityLabel"`
	InitRange       *rangePair `json:"initRange"`
	IndexRange      *rangePair `json:"indexRange"`
	ContentLength   *string    `json:"contentLength"`
}

type rangePair struct {
	Start *string `json:"start"`
	End   *string `json:"end"`
}

func (f playerFormat) url() string {
	return derefString(f.URL)
}

func (f playerFormat) baseURL() string {
	return derefString(f.BaseURL)
}

func (f playerFormat) mimeType() string {
	return derefString(f.MimeType)
}

func (f playerFormat) qualityLabel() string {
	return derefString(f.QualityLabel)
}

func (f playerFormat) height() int {
	if f.Height == nil {
		return 0
	}
	return int(*f.Height)
}

func (f playerFormat) width() int {
	if f.Width == nil {
		return 0
	}
	return int(*f.Width)
}

func (f playerFormat) bitrate() float64 {
	if f.Bitrate == nil {
		return 0
	}
	return *f.Bitrate
}

func (f playerFormat) itagID() int {
	if f.Itag == nil {
		return 0
	}
	return int(*f.Itag)
}

func (r *rangePair) spec() string {
	if r == nil {
		return ""
	}
	return derefString(r.Start) + "-" + derefString(r.End)
}

func (s *streamingData) hls() string {
	if s == nil {
		return ""
	}
	return derefString(s.HLSManifestURL)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// youtubeChannelVideo 频道 Videos tab 条目。
type youtubeChannelVideo struct {
	VideoID       string
	Title         string
	Thumb         string
	ViewsText     string
	PublishedText string
	MembersOnly   bool
}

type youtubeChannelVideosPage struct {
	Videos       []youtubeChannelVideo
	Continuation *string
}

// youtubeChannelLive 频道 Live tab 的「当前在播」定位结果。
type youtubeChannelLive struct {
	VideoID     string
	Thumb       string
	ChannelName string
}

// youtubeLiveHit 直播页/直播搜索的条目。
type youtubeLiveHit struct {
	VideoID      string
	Title        string
	ChannelTitle string
	Thumb        string
	Viewers      *int
}

// youtubeLiveShelf 直播页的一个栏目。
type youtubeLiveShelf struct {
	Title string
	Slug  string
	Hits  []youtubeLiveHit
}

// youtubeChannelHit 搜频道的命中。
type youtubeChannelHit struct {
	ChannelID      string
	Title          string
	Thumb          string
	SubsText       string
	VideoCountText string
}

// liveMeta 直播/视频的真实元信息(取自 player API)。
type liveMeta struct {
	Title  string
	Author string
	IsLive bool
	Online *int
}
