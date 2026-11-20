// douyin 上游记录 → platform.Item 的纯映射(不碰网络;fixture 单测覆盖)。
// 字段语义对齐 TS impl/user-search.ts 的 toItem。

package douyin

import (
	"strings"

	"mygo-host/platform"
)

// userHit 搜索命中的用户(查号与页面捕获两路共用)。
type userHit struct {
	secUID       string
	nickname     string
	avatar       string
	uniqueID     string
	followers    float64
	hasFollowers bool
}

// userItem 用户命中 → Item(url 固定用户主页;ID 取 secUid 末 12 位)。
// Author.handle 供 Profile.Open 解析用户身份(页面按 scope 把 channel 卡路由到主页)。
func userItem(hit userHit, fetchedAt int64) platform.Item {
	item := platform.Item{
		ID:        "dyu-" + lastN(hit.secUID, 12),
		SourceID:  searchSourceID,
		Kind:      platform.KindVideo,
		Title:     hit.nickname,
		URL:       platform.Str("https://www.douyin.com/user/" + hit.secUID),
		FetchedAt: fetchedAt,
		Author: &platform.Author{
			Name:   hit.nickname,
			Avatar: platform.StrOmit(hit.avatar),
			Handle: platform.StrOmit(hit.secUID),
		},
	}
	if hit.avatar != "" {
		item.Thumbnail = platform.Str(hit.avatar)
	}
	if summary := userSummary(hit); summary != "" {
		item.Summary = platform.Str(summary)
	}
	return item
}

// userSummary 「抖音号 x · 1.2万 粉丝」,两者皆空时不输出。
func userSummary(hit userHit) string {
	parts := []string{}
	if hit.uniqueID != "" {
		parts = append(parts, "抖音号 "+hit.uniqueID)
	}
	if hit.hasFollowers {
		parts = append(parts, formatCount(hit.followers)+" 粉丝")
	}
	return strings.Join(parts, " · ")
}
