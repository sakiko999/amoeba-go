// douyin 关键词搜索:浏览器导航搜索页并被动捕获页面自发的用户搜索响应。
// 登录门禁只读 sessionid(ttwid 匿名访客也会签发,不算登录),不向页面注入
// cookie;搜索页一次性使用,Do 返回后立刻 CloseTab 释放 renderer。

package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	douyinOrigin   = "https://www.douyin.com"
	captureIdle    = 1200 * time.Millisecond
	captureTimeout = 20 * time.Second
	closeTimeout   = 5 * time.Second
	// noResultsMessage 捕获窗口内无可用响应(TS NO_RESULTS 原文)。
	noResultsMessage = "抖音搜索没有触发结果请求:可能是页面被风控、关键词无结果,或搜索页结构已变(请在浏览器里确认能正常搜索)"
)

// searchByKeyword 登录门禁 → 导航搜索页捕获 → 解析用户列表。
func searchByKeyword(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]platform.Item, error) {
	if sc.Browser == nil {
		return nil, platform.ErrBrowserUnavailable
	}
	var captured []platform.CapturedResponse
	err := sc.Browser.Do(ctx, douyinOrigin, func(page platform.BrowserPage) error {
		cookies, err := page.Cookies(ctx, douyinOrigin)
		if err != nil {
			return err
		}
		if cookies["sessionid"] == "" {
			return fmt.Errorf("%w: 抖音关键词搜索需要登录,请先在「登录态与密钥」页扫码登录;也可用抖音号直查", platform.ErrBrowserLoginRequired)
		}
		captured, err = page.CaptureNavigate(ctx, searchPageURL(keyword), isSearchAPI, captureIdle, captureTimeout)
		return err
	})
	// CloseTab 必须在 Do 返回后调用(Do 回调内调用会永久死锁)。
	// 搜索超时是常见路径,原 ctx 已取消时仍要关页释放 renderer。
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	if closeErr := sc.Browser.CloseTab(closeCtx, douyinOrigin); closeErr != nil {
		sc.Log("douyin: 关闭搜索页失败: %v", closeErr)
	}
	if err != nil {
		return nil, err
	}
	hits := parseCapturedUsers(captured)
	if len(hits) == 0 {
		return nil, errors.New(noResultsMessage)
	}
	items := make([]platform.Item, 0, len(hits))
	for _, hit := range hits {
		items = append(items, userItem(hit, sc.NowMs()))
	}
	return items, nil
}

// searchPageURL 搜索页 URL(关键词按 encodeURIComponent 编码,type=user)。
func searchPageURL(keyword string) string {
	return douyinOrigin + "/search/" + encodeURIComponent(keyword) + "?type=user"
}

// isSearchAPI 页面自发搜索接口判定:只认 douyin.com 域与 pathname 特征。
func isSearchAPI(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	if host != "douyin.com" && !strings.HasSuffix(host, ".douyin.com") {
		return false
	}
	path := u.Path
	return strings.Contains(path, "/discover/search/") ||
		strings.Contains(path, "/general/search/") ||
		strings.Contains(path, "/search/user/")
}

// parseCapturedUsers 解析两种响应形状,按 secUid 保序去重。
func parseCapturedUsers(responses []platform.CapturedResponse) []userHit {
	hits := []userHit{}
	seen := map[string]bool{}
	for _, res := range responses {
		var payload map[string]any
		if json.Unmarshal([]byte(res.Body), &payload) != nil {
			continue
		}
		for _, list := range userLists(payload) {
			for _, entry := range list {
				raw, _ := entry.(map[string]any)
				hit, ok := userHitFrom(raw)
				if !ok || seen[hit.secUID] {
					continue
				}
				seen[hit.secUID] = true
				hits = append(hits, hit)
			}
		}
	}
	return hits
}

// userLists 顶层 user_list 与 data[].user_list 两种包裹形状。
func userLists(payload map[string]any) [][]any {
	lists := [][]any{}
	if list, ok := payload["user_list"].([]any); ok {
		lists = append(lists, list)
	}
	if data, ok := payload["data"].([]any); ok {
		for _, item := range data {
			entry, _ := item.(map[string]any)
			if list, ok := entry["user_list"].([]any); ok {
				lists = append(lists, list)
			}
		}
	}
	return lists
}

// userHitFrom entry → userHit;user_info 为 null/缺失时退回 entry 自身。
func userHitFrom(entry map[string]any) (userHit, bool) {
	u := entry
	if info, exists := entry["user_info"]; exists && info != nil {
		if m, ok := info.(map[string]any); ok {
			u = m
		} else {
			u = map[string]any{}
		}
	}
	secUID := strings.TrimSpace(platform.JSString(u["sec_uid"]))
	if secUID == "" {
		return userHit{}, false
	}
	hit := userHit{
		secUID:   secUID,
		nickname: strings.TrimSpace(platform.JSString(u["nickname"])),
		avatar:   pickAvatar(u),
		uniqueID: strings.TrimSpace(platform.JSString(u["unique_id"])),
	}
	if hit.nickname == "" {
		hit.nickname = lastN(secUID, 8)
	}
	if n, ok := platform.JSNumber(u["follower_count"]); ok && n != 0 {
		hit.followers, hit.hasFollowers = n, true
	}
	return hit, true
}

// pickAvatar thumb 优先(空串仍算命中,不回落 medium),两者都空则无头像。
func pickAvatar(u map[string]any) string {
	for _, key := range []string{"avatar_thumb", "avatar_medium"} {
		nested, _ := u[key].(map[string]any)
		list, _ := nested["url_list"].([]any)
		if len(list) == 0 || list[0] == nil {
			continue
		}
		return platform.JSString(list[0])
	}
	return ""
}
