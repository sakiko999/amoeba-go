// 抖音扫码登录：直接导航 /login_page（独立登录页，~5s 出码；首页要等 SPA
// 加载再点登录弹窗，实测 11–17s）。二维码取页面内 data:image PNG，按 PNG 头
// 尺寸 ≥200 取最大（页面里还夹着小尺寸 logo）。sessionid 出现即成功。

package douyin

import (
	"context"
	"errors"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const douyinLoginPage = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com"

// 抖音登录页是 SPA，二维码是 data:image PNG，容器 id 为
// #animate_qrcode_container；先收窄到该容器，找不到再退回全页 data:image。
// 仍按 PNG 头尺寸过滤，避免把同页 logo/占位图当成二维码。
const douyinQRSrcExpr = `(() => {
	const pngSize = (src) => {
		try {
			const comma = src.indexOf(',');
			if (comma < 0) return { w: 0, h: 0 };
			const bin = atob(src.slice(comma + 1));
			if (bin.charCodeAt(0) !== 0x89 || bin.slice(1, 4) !== 'PNG') return { w: 0, h: 0 };
			if (bin.slice(12, 16) !== 'IHDR') return { w: 0, h: 0 };
			const u32 = (o) => ((bin.charCodeAt(o) << 24) | (bin.charCodeAt(o + 1) << 16) |
				(bin.charCodeAt(o + 2) << 8) | (bin.charCodeAt(o + 3))) >>> 0;
			return { w: u32(16), h: u32(20) };
		} catch { return { w: 0, h: 0 }; }
	};
	const root = document.querySelector('#animate_qrcode_container') || document.body || document;
	let best = '', bestArea = 0;
	for (const img of root.querySelectorAll('img[src^="data:image"]')) {
		const src = img.currentSrc || img.src || '';
		if (!src.startsWith('data:image/png')) continue;
		const { w, h } = pngSize(src);
		if (w >= 200 && h >= 200 && w * h > bestArea) {
			best = src;
			bestArea = w * h;
		}
	}
	if (best) return best;
	const cv = root.querySelector('canvas');
	if (cv && cv.width >= 200 && cv.height >= 200) {
		try { return cv.toDataURL('image/png'); } catch { return ''; }
	}
	return '';
})()`

// LoginFlow 返回抖音登录配方。
func LoginFlow() login.Flow {
	return login.Flow{
		ID: "douyin", Label: label, Color: "#161823", Hosts: []string{"douyin.com"},
		Origin: "https://www.douyin.com", LoginCookie: "sessionid",
		Timeout: 180 * time.Second, TrustCookies: true, Open: douyinOpen,
	}
}

func douyinOpen(ctx context.Context, env *login.Env, bp platform.BrowserPage) (string, bool, error) {
	// 不走 Navigate：它等 readyState complete，抖音登录页 load 常超过 10s，
	// 而二维码 ~5s 就渲染。location.assign 后按 400ms 轮询，过渡期的
	// evaluate 异常当「未就绪」吞掉（与 WaitUntil 同款语义）。
	before := login.BeginDocNavigation(ctx, bp, douyinLoginPage)
	deadline := time.Now().Add(25 * time.Second)
	attempt := 0
	for time.Now().Before(deadline) {
		if login.DocNavigated(ctx, bp, before) {
			if src := login.EvalString(bp, ctx, douyinQRSrcExpr); src != "" {
				return src, false, nil
			}
		}
		if !login.SleepCtx(ctx, login.QuickPollDelay(attempt)) {
			return "", false, ctx.Err()
		}
		attempt++
	}
	return "", false, errors.New("抖音二维码未出现(登录页结构可能已变)")
}
