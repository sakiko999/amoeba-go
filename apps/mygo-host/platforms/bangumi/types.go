// Bangumi v0 上游响应结构(字段全可选:缺字段时映射层兜底)。
// infobox 的 value 是字符串或 [{v:"..."}] 数组两种形态,保留 RawMessage
// 交映射层判别;数值字段用指针区分「缺字段」与 0。

package bangumi

import "encoding/json"

type rawImages struct {
	Large *string `json:"large"`
}

type rawRating struct {
	Score *float64 `json:"score"`
	Total *int     `json:"total"`
}

type rawInfoboxItem struct {
	Key   *string         `json:"key"`
	Value json.RawMessage `json:"value"`
}

type rawSubject struct {
	ID            *int             `json:"id"`
	Type          *int             `json:"type"`
	Name          *string          `json:"name"`
	NameCn        *string          `json:"name_cn"`
	Summary       *string          `json:"summary"`
	Date          *string          `json:"date"`
	TotalEpisodes *int             `json:"total_episodes"`
	Eps           *int             `json:"eps"`
	Images        *rawImages       `json:"images"`
	Rating        *rawRating       `json:"rating"`
	Infobox       []rawInfoboxItem `json:"infobox"`
}

type rawEpisode struct {
	ID              *int     `json:"id"`
	Ep              *float64 `json:"ep"`
	Sort            *float64 `json:"sort"`
	Name            *string  `json:"name"`
	NameCn          *string  `json:"name_cn"`
	Desc            *string  `json:"desc"`
	Airdate         *string  `json:"airdate"`
	DurationSeconds *float64 `json:"duration_seconds"`
}

type rawSubjectList struct {
	Data []rawSubject `json:"data"`
}

type rawEpisodeList struct {
	Data []rawEpisode `json:"data"`
}
