// 搜索接口被动捕获解析(对齐 TS readSearchCaptured):只按结果包路径过滤,
// 先解析 data.users / data.items[].note_card.user 的精确形状,再对
// usersearch/users 包做深度遍历;结果包存在但没有用户时不回落 SSR。
// 捕获路径与 SSR 投影不同:只解一层 _rawValue,avatar 不展开对象。

package xhs

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"mygo-host/platform"
)

var (
	xhsSearchHostRe = regexp.MustCompile(`(^|\.)xiaohongshu\.com$`)
	resultPackRe    = regexp.MustCompile(`/search/(usersearch|users?|notes)$`)
	userPackRe      = regexp.MustCompile(`/search/(usersearch|users?)$`)
	fansTextRe      = regexp.MustCompile(`^([\d.]+)\s*(万|亿)?$`)
)

// isXhsSearchAPI 页面自发搜索接口:host 为 xiaohongshu.com 或子域,
// pathname 以 /api/sns/web/v1/search/ 开头。
func isXhsSearchAPI(u *url.URL) bool {
	if u == nil || !xhsSearchHostRe.MatchString(strings.ToLower(u.Hostname())) {
		return false
	}
	return strings.HasPrefix(u.Path, "/api/sns/web/v1/search/")
}

// capturedUsers 解析两段捕获的用户;bool = 本次窗口是否出现过结果包。
func capturedUsers(responses []platform.CapturedResponse) ([]xhsUserHit, bool) {
	resultEntries := filterByPath(responses, resultPackRe)
	var hits []xhsUserHit
	for _, resp := range resultEntries {
		hits = append(hits, parseCapturedEntry(resp.Body)...)
	}
	if len(hits) == 0 {
		for _, resp := range filterByPath(responses, userPackRe) {
			walkUsers(parseJSONBody(resp.Body), 0, &hits)
		}
	}
	if len(hits) > 0 {
		return dedupeUsers(hits), true
	}
	if len(resultEntries) == 0 {
		for _, resp := range responses {
			walkUsers(parseJSONBody(resp.Body), 0, &hits)
		}
	}
	return dedupeUsers(hits), len(resultEntries) > 0
}

func filterByPath(responses []platform.CapturedResponse, re *regexp.Regexp) []platform.CapturedResponse {
	out := make([]platform.CapturedResponse, 0, len(responses))
	for _, resp := range responses {
		if re.MatchString(pathOf(resp.URL)) {
			out = append(out, resp)
		}
	}
	return out
}

func pathOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Path
}

// parseCapturedEntry 精确形状:data.users[] 与 data.items[].note_card.user。
func parseCapturedEntry(body string) []xhsUserHit {
	root, ok := parseJSONBody(body).(map[string]any)
	if !ok {
		return nil
	}
	outer, _ := root["data"].(map[string]any)
	if outer == nil {
		return nil
	}
	data := outer
	if !isAnySlice(outer["users"]) && !isAnySlice(outer["items"]) {
		if nested, ok := outer["data"].(map[string]any); ok {
			data = nested
		}
	}
	hits := []xhsUserHit{}
	if users, ok := data["users"].([]any); ok {
		for _, node := range users {
			if hit, ok := userNodeToHit(node); ok {
				hits = append(hits, hit)
			}
		}
	}
	if items, ok := data["items"].([]any); ok {
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			card, _ := item["note_card"].(map[string]any)
			if hit, ok := userNodeToHit(card["user"]); ok {
				hits = append(hits, hit)
			}
		}
	}
	return hits
}

// userNodeToHit 单节点归一:一层 _rawValue、userish 才认 id、note_id 排除。
func userNodeToHit(node any) (xhsUserHit, bool) {
	o, ok := unwrapNode(node)
	if !ok {
		return xhsUserHit{}, false
	}
	userish := hasAnyKey(o, "red_id", "fans", "xsec_token", "note_count", "sub_title")
	idValue := coalesce(o, "user_id", "userId", "userid")
	if idValue == nil && userish {
		idValue = o["id"]
	}
	userID := jsText(idValue)
	nickname := jsText(coalesce(o, "nickname", "name"))
	if userID == "" || nickname == "" {
		return xhsUserHit{}, false
	}
	if platform.JSTruthy(o["note_id"]) || platform.JSTruthy(o["noteId"]) {
		return xhsUserHit{}, false
	}
	hit := xhsUserHit{
		UserID:   userID,
		Nickname: nickname,
		Avatar:   jsText(coalesce(o, "image", "avatar", "imageb")),
	}
	if redID := jsText(coalesce(o, "red_id", "redId")); redID != "" {
		hit.Desc = "小红书号 " + redID
	}
	if fans := parseFans(coalesce(o, "fans", "follower_count", "fans_count")); fans != 0 {
		hit.DescExtra = fmtCount(fans) + " 粉丝"
	}
	return hit, true
}

// walkUsers 深度遍历用户形状对象(上限 10 层);JSON 树无环,不需要 seen。
func walkUsers(node any, depth int, out *[]xhsUserHit) {
	if depth > 10 || node == nil {
		return
	}
	if o, ok := node.(map[string]any); ok {
		if raw, exists := o["_rawValue"]; exists {
			node = raw
		}
	}
	switch v := node.(type) {
	case []any:
		for _, child := range v {
			walkUsers(child, depth+1, out)
		}
	case map[string]any:
		if hit, ok := userNodeToHit(v); ok {
			*out = append(*out, hit)
		}
		for _, child := range v {
			walkUsers(child, depth+1, out)
		}
	}
}

func parseJSONBody(body string) any {
	var v any
	if json.Unmarshal([]byte(body), &v) != nil {
		return nil
	}
	return v
}

func unwrapNode(node any) (map[string]any, bool) {
	o, ok := node.(map[string]any)
	if !ok {
		return nil, false
	}
	if raw, exists := o["_rawValue"]; exists {
		if o, ok = raw.(map[string]any); !ok {
			return nil, false
		}
	}
	return o, true
}

// coalesce TS ?? 语义:取第一个存在且非 nil 的键。
func coalesce(o map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := o[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func hasAnyKey(o map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := o[key]; ok && v != nil {
			return true
		}
	}
	return false
}

func isAnySlice(v any) bool {
	_, ok := v.([]any)
	return ok
}

// jsText TS String();对象按 JS 语义得到 "[object Object]"(捕获路径不解包)。
func jsText(v any) string {
	if _, ok := v.(map[string]any); ok {
		return "[object Object]"
	}
	return platform.JSString(v)
}

// parseFans 数字原样;字符串只认 "29.9万" / "3.4亿" 形态,其余 0。
func parseFans(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	m := fansTextRe.FindStringSubmatch(strings.TrimSpace(platform.JSString(v)))
	if m == nil {
		return 0
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	switch m[2] {
	case "亿":
		return n * 1e8
	case "万":
		return n * 1e4
	}
	return n
}
