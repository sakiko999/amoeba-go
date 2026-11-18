// 媒体元数据契约(与 platform-contracts/src/model.ts 对齐):
// provider 输出、刮削/NFO 与页面共用同一份定义,字段名与 JSON 形状不随实现语言变。

package platform

// MediaType 媒体类型。
type MediaType string

const (
	MediaTV    MediaType = "tv"
	MediaMovie MediaType = "movie"
)

// UniqueId 跨源标识(source 不能取 "default",Kodi 禁止)。
type UniqueId struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

// Rating 评分(0-10 分制)。
type Rating struct {
	Source string  `json:"source"`
	Value  float64 `json:"value"`
	Votes  *int    `json:"votes,omitempty"`
}

// Actor 演职员(order 0 为第一位,Kodi 用 <order> 排序)。
type Actor struct {
	Name     string  `json:"name"`
	Role     *string `json:"role,omitempty"`
	Order    int     `json:"order"`
	ThumbURL *string `json:"thumbUrl,omitempty"`
}

// SearchResult 搜索命中(刻意轻:只够用户从候选里挑一个)。
type SearchResult struct {
	Source        string    `json:"source"`
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle *string   `json:"originalTitle,omitempty"`
	Year          *string   `json:"year,omitempty"`
	MediaType     MediaType `json:"mediaType"`
	PosterURL     *string   `json:"posterUrl,omitempty"`
	Rating        *float64  `json:"rating,omitempty"`
	RatingVotes   *int      `json:"ratingVotes,omitempty"`
	Plot          *string   `json:"plot,omitempty"`
	EpisodeCount  *int      `json:"episodeCount,omitempty"`
}

// EpisodeMetadata 单集元数据(<episodedetails>;aired 是单集播出日期)。
type EpisodeMetadata struct {
	Season         int        `json:"season"`
	Episode        int        `json:"episode"`
	Title          string     `json:"title"`
	OriginalTitle  *string    `json:"originalTitle,omitempty"`
	Plot           *string    `json:"plot,omitempty"`
	Aired          *string    `json:"aired,omitempty"`
	RuntimeMinutes *int       `json:"runtimeMinutes,omitempty"`
	Rating         *Rating    `json:"rating,omitempty"`
	Actors         []Actor    `json:"actors"`
	UniqueIds      []UniqueId `json:"uniqueIds"`
	ThumbURL       *string    `json:"thumbUrl,omitempty"`
}

// MediaMetadata 归一化的作品元数据(只含 NFO writer 会输出的字段)。
type MediaMetadata struct {
	MediaType     MediaType  `json:"mediaType"`
	Title         string     `json:"title"`
	OriginalTitle *string    `json:"originalTitle,omitempty"`
	Plot          *string    `json:"plot,omitempty"`
	Premiered     *string    `json:"premiered,omitempty"` // yyyy-mm-dd
	Genres        []string   `json:"genres"`
	Studios       []string   `json:"studios"`
	Status        *string    `json:"status,omitempty"`
	Ratings       []Rating   `json:"ratings"`
	Actors        []Actor    `json:"actors"`
	UniqueIds     []UniqueId `json:"uniqueIds"`
	PosterURL     *string    `json:"posterUrl,omitempty"`
	SeasonCount   *int       `json:"seasonCount,omitempty"`
	EpisodeCount  *int       `json:"episodeCount,omitempty"`
}

// NewMediaMetadata 建空元数据(集合字段初始化,对应 TS newMetadata)。
func NewMediaMetadata(mediaType MediaType, title string) *MediaMetadata {
	return &MediaMetadata{
		MediaType: mediaType,
		Title:     title,
		Genres:    []string{},
		Studios:   []string{},
		Ratings:   []Rating{},
		Actors:    []Actor{},
		UniqueIds: []UniqueId{},
	}
}

// MetadataYear 从 premiered 切年份(前 4 位全数字才有效)。
func MetadataYear(meta *MediaMetadata) (string, bool) {
	if meta.Premiered == nil || len(*meta.Premiered) < 4 {
		return "", false
	}
	y := (*meta.Premiered)[:4]
	for _, c := range y {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return y, true
}
