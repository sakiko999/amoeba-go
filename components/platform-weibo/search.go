// 用户搜索的浏览器被动采集:导航 m.weibo.cn 搜索页,捕获页面自己发出的
// /api/container/getIndex(应用重放会被 ok:-100 风控;页面请求带 visitor
// 身份与真实指纹,匿名即可用)。搜索页是一次性 target:Do 返回后立即关闭
// 释放 renderer;禁止在 Do 回调内调 CloseTab/Do(platform.Browser 约定)。

package weibo

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	searchOrigin   = "https://m.weibo.cn"
	captureIdle    = 2 * time.Second
	captureTimeout = 25 * time.Second
	closeTimeout   = 5 * time.Second

	needsBrowserMessage = "微博搜用户需要桌面抓取浏览器：搜索接口只由页面自己发（应用重放会触发 -100 风控）"
	riskMessage         = "微博用户搜索失败: ok=-100 请求过于频繁，请过几分钟再试"
	noResultsMessage    = "微博搜索没有触发结果请求：可能是页面被风控、关键词无结果，或搜索页结构已变（请在浏览器里确认能正常搜索）"
)

var searchHostRe = regexp.MustCompile(`(^|\.)m\.weibo\.cn$`)

// searchUsers 一次完整搜索:载具判空 → 捕获 → 关页 → 解析 → 映射。
func searchUsers(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]platform.Item, error) {
	if sc.Browser == nil {
		return nil, fmt.Errorf("%w: %s", platform.ErrBrowserUnavailable, needsBrowserMessage)
	}
	captured, err := captureUserSearch(ctx, sc, keyword)
	if err != nil {
		return nil, err
	}
	hits, found, risk := parseUserSearch(captured, keyword)
	if found && len(hits) > 0 {
		return userSearchItems(hits, sc.NowMs()), nil
	}
	if !found && risk {
		return nil, fmt.Errorf("%w: %s", platform.ErrBrowserRiskControl, riskMessage)
	}
	return nil, errors.New(noResultsMessage)
}

// captureUserSearch 在 m.weibo.cn target 上捕获页面自发的用户搜索响应。
func captureUserSearch(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]platform.CapturedResponse, error) {
	var captured []platform.CapturedResponse
	err := sc.Browser.Do(ctx, searchOrigin, func(page platform.BrowserPage) error {
		var captureErr error
		captured, captureErr = page.CaptureNavigate(ctx, searchPageURL(keyword),
			isWeiboUserSearchAPI, captureIdle, captureTimeout)
		return captureErr
	})
	closeSearchTab(ctx, sc)
	return captured, err
}

// closeSearchTab 尽力释放一次性搜索页;失败只记日志,不覆盖搜索错误。
func closeSearchTab(ctx context.Context, sc *platform.ServiceContext) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	if err := sc.Browser.CloseTab(closeCtx, searchOrigin); err != nil {
		sc.Log("weibo: 关闭搜索页失败: %v", err)
	}
}

// searchPageURL 复刻 TS 模板:containerid 整段 query 编码,内层 &/= 保留为
// %26/%3D(不能变成外层分隔符)。空格编码成 %20/+ 均能被页面按 query 解码。
func searchPageURL(keyword string) string {
	return searchOrigin + "/search?containerid=" + url.QueryEscape("100103type=3&q="+keyword)
}

// isWeiboUserSearchAPI 匹配页面自己的用户搜索接口(type=3);不看 method,
// 只认 containerid 前缀,容忍页面追加 display/retcode 等参数。
func isWeiboUserSearchAPI(u *url.URL) bool {
	if u == nil || !searchHostRe.MatchString(strings.ToLower(u.Hostname())) {
		return false
	}
	if u.Path != "/api/container/getIndex" {
		return false
	}
	return strings.HasPrefix(u.Query().Get("containerid"), "100103type=3")
}
