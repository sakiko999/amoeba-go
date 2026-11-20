// getIndex 捕获体解析:严格复刻 TS readCaptured 的语义——URL 解码后的
// containerid 必须包含 q=<kw>(includes,容忍页面追加参数);JSON 解析失败
// 跳过;首个 ok===1 立即返回用户卡;-100 只记风控标志,不能当「0 条」吞掉。
// 捕获窗口是调用级的,进程级 passiveBuffer 的 since 过滤在此天然成立。

package weibo

import (
	"encoding/json"
	"net/url"
	"strings"

	"mygo-host/platform"
)

// weiboUserHit 用户卡归一结果(followers 保持上游格式化串,如 "207.7万")。
type weiboUserHit struct {
	UID        string
	ScreenName string
	Avatar     string
	Followers  string
	Verified   string
}

// parseUserSearch 按捕获完成顺序扫描;found 表示见到 ok=1(即使 0 条)。
func parseUserSearch(responses []platform.CapturedResponse, keyword string) (hits []weiboUserHit, found, risk bool) {
	target := "q=" + keyword
	for _, resp := range responses {
		if !urlHasKeyword(resp.URL, target) {
			continue
		}
		var top map[string]any
		if err := json.Unmarshal([]byte(resp.Body), &top); err != nil {
			continue
		}
		ok, _ := top["ok"].(float64)
		switch ok {
		case 1:
			return hitsFromTop(top), true, risk
		case -100:
			risk = true
		}
	}
	return nil, false, risk
}

// urlHasKeyword containerid 取解码后的 query 值,includes 语义而非前缀。
func urlHasKeyword(rawURL, target string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.Contains(u.Query().Get("containerid"), target)
}

func hitsFromTop(top map[string]any) []weiboUserHit {
	data, _ := top["data"].(map[string]any)
	cards, _ := data["cards"].([]any)
	return hitsFromCards(cards)
}

// hitsFromCards cards → card_group[].user / card.user(两种形态都收);
// 组卡只要 card_group 是数组就优先使用,此时该 card 自身的 user 不再看。
func hitsFromCards(cards []any) []weiboUserHit {
	hits := make([]weiboUserHit, 0, len(cards))
	seen := map[string]bool{}
	for _, raw := range cards {
		card, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		group, ok := card["card_group"].([]any)
		if !ok {
			group = []any{card}
		}
		for _, entry := range group {
			hit, ok := userHit(entry)
			if !ok || seen[hit.UID] {
				continue
			}
			seen[hit.UID] = true
			hits = append(hits, hit)
		}
	}
	return hits
}

// userHit 单卡归一:uid/screen_name 非空才保留;同响应内按 uid 去重。
func userHit(raw any) (weiboUserHit, bool) {
	item, ok := raw.(map[string]any)
	if !ok {
		return weiboUserHit{}, false
	}
	user, ok := item["user"].(map[string]any)
	if !ok {
		return weiboUserHit{}, false
	}
	uid := strings.TrimSpace(platform.JSString(user["id"]))
	name := strings.TrimSpace(platform.JSString(user["screen_name"]))
	if uid == "" || name == "" {
		return weiboUserHit{}, false
	}
	return weiboUserHit{
		UID:        uid,
		ScreenName: name,
		Avatar:     avatarOf(user),
		Followers:  platform.JSString(user["followers_count"]),
		Verified:   strings.TrimSpace(platform.JSString(user["verified_reason"])),
	}, true
}

// avatarOf 复刻 ?? 语义:只有 avatar_hd 为 null/缺失才回退 profile_image_url,
// 空串不回退(最终空串由映射层省略)。
func avatarOf(user map[string]any) string {
	value := user["avatar_hd"]
	if value == nil {
		value = user["profile_image_url"]
	}
	return platform.JSString(value)
}
