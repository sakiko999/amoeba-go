// 小红书扫码登录：/login 独立页直接挂二维码，不需要点首页「登录」按钮；
// 二维码是 img.qrcode-img 的 data URL（canvas 兜底）。登录态不能用
// web_session（游客也签发），初始只看页面 SSR user.loggedIn / 侧栏「我」；
// 扫码后以 user_id 相对基线变化或页面态翻转判定。

package xhs

import (
	"context"
	"errors"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const (
	xhsHome = "https://www.xiaohongshu.com"
	// 独立登录页会立即弹出扫码面板；直接用这个入口，省掉首页「登录」按钮。
	xhsLoginPage = "https://www.xiaohongshu.com/login"
	xhsQRObjExpr = `(() => {
		const img = document.querySelector('img.qrcode-img');
		let src = img && img.src ? img.src : '';
		if (!src) {
			const cv = document.querySelector('canvas.qrcode');
			if (cv) { try { src = cv.toDataURL('image/png'); } catch { src = ''; } }
		}
		const expired = /二维码已过期|点击刷新/.test(document.body ? document.body.innerText || '' : '');
		return { src, expired };
	})()`
	xhsPageLoggedExpr = `(() => {
		const u = window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user;
		const l = u && u.loggedIn && (u.loggedIn._rawValue !== undefined ? u.loggedIn._rawValue : u.loggedIn);
		return typeof l === 'boolean' ? l : null;
	})()`
	xhsDomLoggedExpr = `(() => {
		const a = [...document.querySelectorAll('a[href*="/user/profile/"]')]
			.find((n) => n.textContent && n.textContent.trim() === '我');
		return !!a;
	})()`
)

// LoginFlow 返回小红书登录配方。
func LoginFlow() login.Flow {
	return login.Flow{
		ID: "xhs", Label: label, Color: "#ff2442", Hosts: []string{"xiaohongshu.com"},
		Origin: xhsHome, LoginCookie: "user_id",
		BaselineCookie: "user_id",
		Timeout:        300 * time.Second, TrustCookies: false,
		Open: xhsOpen, PollCheck: xhsPollCheck, UserHint: xhsUser,
	}
}

func xhsOpen(ctx context.Context, env *login.Env, bp platform.BrowserPage) (string, bool, error) {
	for range 3 {
		before := login.BeginDocNavigation(ctx, bp, xhsLoginPage)
		deadline := time.Now().Add(10 * time.Second)
		attempt := 0
		for time.Now().Before(deadline) {
			if !login.DocNavigated(ctx, bp, before) {
				if !login.SleepCtx(ctx, login.QuickPollDelay(attempt)) {
					return "", false, ctx.Err()
				}
				attempt++
				continue
			}
			if src, expired := xhsReadQR(bp, ctx); src != "" && !expired {
				return src, false, nil
			}
			if xhsLoggedInPage(bp, ctx) {
				return "", true, nil
			}
			if !login.SleepCtx(ctx, login.QuickPollDelay(attempt)) {
				return "", false, ctx.Err()
			}
			attempt++
		}
	}
	return "", false, errors.New("小红书二维码未出现(登录弹窗可能未打开)")
}

func xhsReadQR(bp platform.BrowserPage, ctx context.Context) (src string, expired bool) {
	v, err := bp.Evaluate(ctx, xhsQRObjExpr)
	if err != nil {
		return "", false
	}
	obj, _ := v.(map[string]any)
	src, _ = obj["src"].(string)
	expired, _ = obj["expired"].(bool)
	return src, expired
}

// xhsLoggedInPage 页面态判登录（SSR 优先，DOM「我」兜底）。
func xhsLoggedInPage(bp platform.BrowserPage, ctx context.Context) bool {
	if v, ok := login.EvalBool(bp, ctx, xhsPageLoggedExpr); ok {
		return v
	}
	v, _ := login.EvalBool(bp, ctx, xhsDomLoggedExpr)
	return v
}

// xhsPollCheck user_id 从基线变化（新会话）或页面态翻转为真（同账号重登）。
func xhsPollCheck(ctx context.Context, env *login.Env, sess *login.Session, cookies map[string]string) (bool, string) {
	uid := cookies["user_id"]
	if uid != "" && uid != sess.BaselineUserID {
		return true, uid
	}
	// 短暂等登录锁：用户点了刷新时 DoAs 会排队，不能把整轮轮询拖住。
	pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var logged bool
	_ = env.Browser.DoAs(pctx, login.Key("xhs"), xhsHome, func(bp platform.BrowserPage) error {
		logged = xhsLoggedInPage(bp, pctx)
		return nil
	})
	return logged, uid
}

func xhsUser(cookies map[string]string) string { return cookies["user_id"] }
