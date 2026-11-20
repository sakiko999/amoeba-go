// 豆瓣扫码登录的浏览器回退：HTTP 取码失败时才走这里。登录页点
// .quick.icon-switch 切到「豆瓣 App 扫码登录」面板，二维码是
// .account-qr-code 里的客户端生成 data URL（195×195）。

package douban

import (
	"context"
	"errors"
	"strings"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const (
	doubanLoginURL   = "https://accounts.douban.com/passport/login"
	doubanSwitchExpr = `(() => {
		const img = document.querySelector('.account-qr-code .account-qr-scan img');
		if (img && img.src) return false;
		const el = document.querySelector('.quick.icon-switch');
		if (!el || getComputedStyle(el).display === 'none') return false;
		el.click();
		return true;
	})()`
	doubanQRSrcExpr = `(() => {
		const img = document.querySelector('.account-qr-code .account-qr-scan img');
		return img && img.src ? img.src : '';
	})()`
)

func doubanOpen(ctx context.Context, env *login.Env, bp platform.BrowserPage) (string, bool, error) {
	// 同 douyin：不等 load 事件，location.assign 后轮询。switch 表达式自带
	// 守卫（二维码已在/开关隐藏时不点击），每轮重试安全。
	_, _ = bp.Evaluate(ctx, `location.assign('`+doubanLoginURL+`')`)
	deadline := time.Now().Add(25 * time.Second)
	attempt := 0
	for time.Now().Before(deadline) {
		_, _ = login.EvalBool(bp, ctx, doubanSwitchExpr)
		if src := login.EvalString(bp, ctx, doubanQRSrcExpr); src != "" {
			// data URL 原样返回；远程图由宿主抓取并校验，失败继续等而不是显示空框。
			if qr := env.FetchImage(ctx, src, "https://accounts.douban.com/"); qr != "" {
				return qr, false, nil
			}
		}
		if !login.SleepCtx(ctx, login.QuickPollDelay(attempt)) {
			return "", false, ctx.Err()
		}
		attempt++
	}
	return "", false, errors.New("豆瓣二维码未出现(登录页结构可能已变)")
}

func doubanUser(cookies map[string]string) string {
	v := cookies["dbcl2"]
	if i := strings.IndexByte(v, ':'); i > 0 {
		return v[:i]
	}
	return v
}
