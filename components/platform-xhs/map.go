// 用户命中(SSR 投影或被动捕获)→ platform.Item 的纯映射(不碰网络)。
// kind 固定 video 对齐 TS 渠道 xhs:user:search;url 指向主页,下游从 url
// 解析 user_id 并订阅 xhs:user。summary 由「小红书号」与粉丝数用「 · 」连接。

package xhs

import (
	"strconv"
	"strings"

	"mygo-host/platform"
)

// projectedUser 是页面内 SSR 投影表达式输出的标量结构。
type projectedUser struct {
	UserID   string  `json:"userId"`
	Nickname string  `json:"nickname"`
	Avatar   string  `json:"avatar"`
	RedID    string  `json:"redId"`
	Fans     float64 `json:"fans"`
}

// xhsUserHit 投影后的用户候选。
type xhsUserHit struct {
	UserID    string
	Nickname  string
	Avatar    string
	Desc      string
	DescExtra string
}

func projectUsers(list []projectedUser) []xhsUserHit {
	hits := make([]xhsUserHit, 0, len(list))
	for _, u := range list {
		hit := xhsUserHit{UserID: u.UserID, Nickname: u.Nickname, Avatar: u.Avatar}
		if u.RedID != "" {
			hit.Desc = "小红书号 " + u.RedID
		}
		if u.Fans != 0 {
			hit.DescExtra = fmtCount(u.Fans) + " 粉丝"
		}
		hits = append(hits, hit)
	}
	return hits
}

// dedupeUsers 按 userId 去重(保留首次出现),缺 id/昵称的条目丢弃。
func dedupeUsers(list []xhsUserHit) []xhsUserHit {
	seen := make(map[string]struct{}, len(list))
	out := make([]xhsUserHit, 0, len(list))
	for _, hit := range list {
		if hit.UserID == "" || hit.Nickname == "" {
			continue
		}
		if _, dup := seen[hit.UserID]; dup {
			continue
		}
		seen[hit.UserID] = struct{}{}
		out = append(out, hit)
	}
	return out
}

func userToItem(hit xhsUserHit, fetchedAt int64) platform.Item {
	id := hit.UserID
	if id == "" {
		id = hit.Nickname
	}
	return platform.Item{
		ID:        "xu-" + id,
		SourceID:  sourceID,
		Kind:      platform.KindVideo,
		Title:     hit.Nickname,
		URL:       platform.Str(xhsBase + "/user/profile/" + hit.UserID),
		Thumbnail: platform.StrOmit(hit.Avatar),
		Summary:   summaryOf(hit),
		FetchedAt: fetchedAt,
		Author: &platform.Author{
			Name:   hit.Nickname,
			Avatar: platform.StrOmit(hit.Avatar),
			Handle: platform.StrOmit(hit.UserID),
		},
	}
}

// summaryOf 「小红书号 xxx · 1.2万 粉丝」,空段省略。
func summaryOf(hit xhsUserHit) *string {
	parts := make([]string, 0, 2)
	if hit.Desc != "" {
		parts = append(parts, hit.Desc)
	}
	if hit.DescExtra != "" {
		parts = append(parts, hit.DescExtra)
	}
	return platform.StrOmit(strings.Join(parts, " · "))
}

// fmtCount 粉丝数缩写(对齐 TS toFixed(1) 后裁掉 .0):1.2万 / 3.4亿。
func fmtCount(n float64) string {
	if n >= 1e8 {
		return trimPointZero(strconv.FormatFloat(n/1e8, 'f', 1, 64)) + "亿"
	}
	if n >= 1e4 {
		return trimPointZero(strconv.FormatFloat(n/1e4, 'f', 1, 64)) + "万"
	}
	return platform.JSString(n)
}

func trimPointZero(s string) string {
	return strings.TrimSuffix(s, ".0")
}
