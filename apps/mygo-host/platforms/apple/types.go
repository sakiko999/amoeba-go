// Apple iTunes 目录 API 的响应结构(字段全部可选)与归一后的播客条目。
// 榜单是旧 iTunes RSS 形态,Lookup/Search 是普通 JSON,两者共用 podcast。

package apple

import (
	"bytes"
	"encoding/json"
)

// podcast 归一后的节目条目(榜单 / 搜索 / Lookup 三路共用)。
type podcast struct {
	ID           int64
	Title        string
	Author       string
	FeedURL      string
	Artwork      *string
	PageURL      *string
	Genre        *string
	EpisodeCount *int
}

// textLabel 上游大量使用的 {label} 包装。
type textLabel struct {
	Label *string `json:"label"`
}

type chartEntryID struct {
	Attributes *struct {
		IMID *string `json:"im:id"`
	} `json:"attributes"`
}

type chartImage struct {
	Label      *string `json:"label"`
	Attributes *struct {
		Height *string `json:"height"`
	} `json:"attributes"`
}

type chartCategory struct {
	Attributes *textLabel `json:"attributes"`
}

// chartEntry 旧 iTunes 榜单条目(只取映射用到的字段)。
type chartEntry struct {
	ID       *chartEntryID         `json:"id"`
	Images   oneOrMany[chartImage] `json:"im:image"`
	Category *chartCategory        `json:"category"`
}

type chartResponse struct {
	Feed *struct {
		Entry oneOrMany[chartEntry] `json:"entry"`
	} `json:"feed"`
}

// lookupResult Lookup / Search 结果条目。
type lookupResult struct {
	CollectionID      *int64  `json:"collectionId"`
	CollectionName    *string `json:"collectionName"`
	ArtistName        *string `json:"artistName"`
	FeedURL           *string `json:"feedUrl"`
	ArtworkURL600     *string `json:"artworkUrl600"`
	ArtworkURL100     *string `json:"artworkUrl100"`
	CollectionViewURL *string `json:"collectionViewUrl"`
	PrimaryGenreName  *string `json:"primaryGenreName"`
	TrackCount        *int    `json:"trackCount"`
}

type lookupResponse struct {
	Results []lookupResult `json:"results"`
}

// oneOrMany 兼容上游「单个对象 / 数组 / 缺省」三种形态(榜单 entry 两种都有)。
type oneOrMany[T any] []T

func (m *oneOrMany[T]) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '[' {
		var list []T
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}
		*m = list
		return nil
	}
	var single T
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}
	*m = oneOrMany[T]{single}
	return nil
}
