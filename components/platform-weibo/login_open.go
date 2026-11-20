// 微博浏览器回退：passport 页自带二维码 img，图片经宿主 HTTP 转 data URL。

package weibo

import (
	"context"
	"errors"
	"strings"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const weiboLoginURL = "https://passport.weibo.com/sso/signin?entry=miniblog&source=miniblog&disp=popup&url=https%3A%2F%2Fweibo.com"

const weiboQRSrcExpr = `(() => {
		const img = [...document.querySelectorAll('img')].find(
			(i) => /v2\.qr\.weibo\.cn|qr\.weibo\.cn/.test(i.src || '') || /inf\/gen/.test(i.src || '')
		);
		return img && img.src ? img.src : '';
	})()`

// weiboOpen 是微博浏览器回退入口。
func weiboOpen(ctx context.Context, env *login.Env, bp platform.BrowserPage) (string, bool, error) {
	if err := bp.Navigate(ctx, weiboLoginURL); err != nil {
		return "", false, err
	}
	deadline := time.Now().Add(25 * time.Second)
	attempt := 0
	for time.Now().Before(deadline) {
		if src := login.EvalString(bp, ctx, weiboQRSrcExpr); src != "" {
			return env.FetchImage(ctx, src, "https://passport.weibo.com/"), false, nil
		}
		// 已有登录态时通行证页会直接跳回微博主页（并刷新 ALF），交给上方重读 cookie。
		if href := login.EvalString(bp, ctx, "location.href"); strings.Contains(href, "weibo.com") &&
			!strings.Contains(href, "passport.weibo.com") {
			return "", true, nil
		}
		if !login.SleepCtx(ctx, login.QuickPollDelay(attempt)) {
			return "", false, ctx.Err()
		}
		attempt++
	}
	return "", false, errors.New("微博二维码未出现(登录页结构可能已变)")
}
