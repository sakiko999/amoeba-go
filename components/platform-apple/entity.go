// apple entity 搜索结果的 raw 字段;feed 渠道仍保留 RSS 订阅地址。

package apple

import "encoding/json"

type entityRaw struct {
	AppleID int64  `json:"appleId"`
	PageURL string `json:"pageUrl"`
	FeedURL string `json:"feedUrl,omitempty"`
}

func entityRawJSON(p podcast) (json.RawMessage, error) {
	return json.Marshal(entityRaw{
		AppleID: p.ID,
		PageURL: *p.PageURL,
		FeedURL: p.FeedURL,
	})
}
