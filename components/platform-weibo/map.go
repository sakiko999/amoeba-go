// 用户卡 → platform.Item 的纯映射(不碰网络;fixture 单测覆盖)。
// kind 固定 video 对齐 TS 渠道声明;goTpl §4 的 channel 口径(可选
// liveStatus/roomId)不适用于微博用户搜索,按 weibo.md §4.2 以规格为准。
// Author.handle 供 Profile.Open 解析用户身份(entity 结果整卡进主页)。

package weibo

import (
	"strings"

	"mygo-host/platform"
)

// userSearchItems hits → items(所有条目共享同一次 fetch 的 fetchedAt)。
func userSearchItems(hits []weiboUserHit, fetchedAt int64) []platform.Item {
	items := make([]platform.Item, 0, len(hits))
	for _, hit := range hits {
		items = append(items, userSearchItem(hit, fetchedAt))
	}
	return items
}

func userSearchItem(hit weiboUserHit, fetchedAt int64) platform.Item {
	return platform.Item{
		ID:        "wbu-" + hit.UID,
		SourceID:  "weibo:user:search",
		Kind:      platform.KindVideo,
		Title:     hit.ScreenName,
		URL:       platform.Str("https://weibo.com/u/" + hit.UID),
		Thumbnail: platform.StrOmit(hit.Avatar),
		Summary:   summaryOf(hit),
		FetchedAt: fetchedAt,
		Author: &platform.Author{
			Name:   hit.ScreenName,
			Avatar: platform.StrOmit(hit.Avatar),
			Handle: platform.StrOmit(hit.UID),
		},
	}
}

// summaryOf 认证理由 + 「N 关注者」,空段省略,「 · 」连接。
func summaryOf(hit weiboUserHit) *string {
	parts := make([]string, 0, 2)
	if hit.Verified != "" {
		parts = append(parts, hit.Verified)
	}
	if hit.Followers != "" {
		parts = append(parts, hit.Followers+" 关注者")
	}
	return platform.StrOmit(strings.Join(parts, " · "))
}
