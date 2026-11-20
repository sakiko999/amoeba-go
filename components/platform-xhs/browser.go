// 关键词搜用户的浏览器执行面(规格 §3.10):导航并被动捕获搜索响应 →
// 风控/登录闸门 → 点击「用户」Tab 再捕获 → 解析捕获体;本次窗口没有结果包
// 才回落 __INITIAL_STATE__ SSR。登录态支持下一阶段落地:未登录一律返回
// ErrBrowserLoginRequired,不注入 cookie。按用户 ID / 主页链接查号(§6)
// 留待下一阶段。禁止在 Do 回调内调 CloseTab/Do(platform.Browser 约定)。

package xhs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	xhsBase             = "https://www.xiaohongshu.com"
	sourceID            = "xhs:user:search"
	captureNavIdle      = 2500 * time.Millisecond
	captureNavTimeout   = 20 * time.Second
	captureClickIdle    = 2 * time.Second
	captureClickTimeout = 8 * time.Second
	closeTimeout        = 5 * time.Second
)

// searchUsers 一次完整搜索:载具判空 → Do 内取数 → 关页 → 映射。
func searchUsers(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]platform.Item, error) {
	if sc.Browser == nil {
		return nil, fmt.Errorf("%w: 小红书关键词搜索需要桌面抓取浏览器(搜索接口需要页面签名)", platform.ErrBrowserUnavailable)
	}
	var hits []xhsUserHit
	err := sc.Browser.Do(ctx, xhsBase, func(page platform.BrowserPage) error {
		var collectErr error
		hits, collectErr = collectSearchUsers(ctx, page, sc, keyword)
		return collectErr
	})
	closeSearchTab(ctx, sc)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("没有搜到与「%s」相关的用户：换个关键词，或确认搜索页能正常打开（可能需要登录）", keyword)
	}
	items := make([]platform.Item, 0, len(hits))
	for _, hit := range hits {
		items = append(items, userToItem(hit, sc.NowMs()))
	}
	return items, nil
}

// collectSearchUsers 在搜索页 target 上按规格顺序执行各步骤。
func collectSearchUsers(ctx context.Context, page platform.BrowserPage, sc *platform.ServiceContext, keyword string) ([]xhsUserHit, error) {
	responses, err := page.CaptureNavigate(ctx, searchURL(keyword), isXhsSearchAPI,
		captureNavIdle, captureNavTimeout)
	if err != nil {
		sc.Log("xhs: 首次捕获失败: %v", err)
	}
	if err := assertNotBlocked(ctx, page); err != nil {
		return nil, err
	}
	if err := assertLoggedIn(ctx, page); err != nil {
		return nil, err
	}
	clicked, err := page.Capture(ctx, clickUserTabTrigger(ctx, page), isXhsSearchAPI,
		captureClickIdle, captureClickTimeout)
	if err != nil {
		sc.Log("xhs: 用户 Tab 捕获失败: %v", err)
	}
	hits, hasResultPack := capturedUsers(append(responses, clicked...))
	if len(hits) > 0 || hasResultPack {
		return hits, nil
	}
	return readStateUsers(ctx, page)
}

// clickUserTabTrigger 点击「用户」Tab 作为第二段捕获的触发;找不到时返回 false,
// 捕获按 idle 安静结束。
func clickUserTabTrigger(ctx context.Context, page platform.BrowserPage) func() error {
	return func() error {
		_, err := page.Evaluate(ctx, clickUserTabExpr)
		return err
	}
}

// searchURL 对齐 TS:`type=51` 即用户 Tab 的搜索页。
func searchURL(keyword string) string {
	return xhsBase + "/search_result?keyword=" + encodeURIComponent(keyword) +
		"&source=web_search_result_notes&type=51"
}

// encodeURIComponent 复刻 JS:空格编码为 %20(Go QueryEscape 用 +)。
func encodeURIComponent(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// assertNotBlocked 风控框是明确的平台拦截信号;顺序必须先于登录判定。
func assertNotBlocked(ctx context.Context, page platform.BrowserPage) error {
	blocked, err := evalBool(ctx, page, riskBoxExpr)
	if err != nil {
		return err
	}
	if !blocked {
		return nil
	}
	return fmt.Errorf("%w: 小红书风控校验(.fe-verify-box)出现——请稍后再试或重新扫码登录", platform.ErrBrowserRiskControl)
}

// assertLoggedIn 登录页优先于页面登录位:失效会话跳登录页时 SSR 可能残留 loggedIn=true。
func assertLoggedIn(ctx context.Context, page platform.BrowserPage) error {
	onLoginPage, err := evalBool(ctx, page, loginPageExpr)
	if err != nil {
		return err
	}
	if onLoginPage || !isLoggedIn(ctx, page) {
		return fmt.Errorf("%w: 小红书需要登录：请在「登录态与密钥」页扫码登录后再试", platform.ErrBrowserLoginRequired)
	}
	return nil
}

// isLoggedIn 登录判定链:页面 SSR loggedIn → 侧栏「我」→ user_id cookie。
// web_session 游客也会签发,不能作为登录标志;各步读取失败按「该步不成立」处理。
func isLoggedIn(ctx context.Context, page platform.BrowserPage) bool {
	if raw, err := page.Evaluate(ctx, pageLoggedInExpr); err == nil {
		if loggedIn, ok := raw.(bool); ok {
			return loggedIn
		}
	}
	if loggedIn, err := evalBool(ctx, page, domLoggedInExpr); err == nil && loggedIn {
		return true
	}
	cookies, err := page.Cookies(ctx, xhsBase)
	if err != nil {
		return false
	}
	return cookies["user_id"] != ""
}

// readStateUsers 执行 SSR 投影并解析 JSON 字符串;空投影视为没有用户。
func readStateUsers(ctx context.Context, page platform.BrowserPage) ([]xhsUserHit, error) {
	raw, err := page.Evaluate(ctx, ssrUsersExpr)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	text, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("xhs SSR 投影返回类型异常(%T)", raw)
	}
	if text == "" {
		return nil, nil
	}
	var projected []projectedUser
	if err := json.Unmarshal([]byte(text), &projected); err != nil {
		return nil, fmt.Errorf("xhs SSR 投影解析失败: %w", err)
	}
	return projectUsers(projected), nil
}

func evalBool(ctx context.Context, page platform.BrowserPage, expression string) (bool, error) {
	raw, err := page.Evaluate(ctx, expression)
	if err != nil {
		return false, err
	}
	value, _ := raw.(bool)
	return value, nil
}

// closeSearchTab 尽力释放一次性搜索页(renderer 约 150–260MB),不覆盖搜索错误。
func closeSearchTab(ctx context.Context, sc *platform.ServiceContext) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	if err := sc.Browser.CloseTab(closeCtx, xhsBase); err != nil {
		sc.Log("xhs: 关闭搜索页失败: %v", err)
	}
}
