// 微博纯 HTTP 扫码：新版 passport/sso/v2 取 qrid 与二维码内容，扫码确认后
// 用 check 返回的 data.url 换跨域 ticket，cookie 落在 weibo.com 与 sina.com.cn。

package weibo

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const (
	weiboHTTPBase    = "https://passport.weibo.com"
	weiboQRImagePath = "/sso/v2/qrcode/image"
	weiboQRCheckPath = "/sso/v2/qrcode/check"
	weiboQRReferer   = "https://passport.weibo.com/sso/signin?entry=miniblog&source=miniblog&url=https%3A%2F%2Fweibo.com%2F"
	weiboPollGap     = 2 * time.Second
	weiboRetSuccess  = 20000000
	weiboRetScanned  = 50114002
	weiboRetExpired1 = 50114003
	weiboRetExpired2 = 50114004
)

type weiboHTTPSession struct {
	qrid     string
	cookies  map[string]string
	lastPoll time.Time
}

type weiboEnvelope struct {
	Retcode int    `json:"retcode"`
	Msg     string `json:"msg"`
	Data    struct {
		QRID  string `json:"qrid"`
		Image string `json:"image"`
		URL   string `json:"url"`
		Alt   string `json:"alt"`
	} `json:"data"`
}

func weiboHTTPHeaders(cookies map[string]string) map[string]string {
	h := map[string]string{
		"referer": weiboQRReferer, "origin": weiboHTTPBase,
		"accept": "application/json, text/plain, */*", "x-requested-with": "XMLHttpRequest",
	}
	if ck := login.CookieHeader(cookies); ck != "" {
		h["cookie"] = ck
	}
	return h
}

func weiboHTTPGet(ctx context.Context, env *login.Env, sess *weiboHTTPSession, target string) (*platform.Response, error) {
	cctx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	res, err := login.GetNoRedirect(cctx, env.HTTP, target, weiboHTTPHeaders(sess.cookies))
	if err != nil {
		return nil, err
	}
	login.MergeSetCookies(sess.cookies, res.Headers["set-cookie"])
	return res, nil
}

// weiboHTTPGetFollow 手动跟随重定向，逐跳合并 Set-Cookie；跨域登录不能交给
// http.Client 自动跳，否则中间响应的 cookie 会丢。
func weiboHTTPGetFollow(ctx context.Context, env *login.Env, sess *weiboHTTPSession, target string) (*platform.Response, error) {
	res, err := weiboHTTPGet(ctx, env, sess, target)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 5 && res.Status >= 300 && res.Status < 400; i++ {
		next, err := login.ResolveRedirect(target, res.Headers["location"])
		if err != nil {
			return res, err
		}
		target = next
		if res, err = weiboHTTPGet(ctx, env, sess, target); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// weiboHTTPStart 直接申请二维码；image/check 不需要登录页的 X-CSRF-TOKEN。
func weiboHTTPStart(ctx context.Context, env *login.Env, sess *login.Session) (string, string, error) {
	if env.HTTP == nil {
		return "", "", errors.New("weibo 扫码: 缺少 HTTP 客户端")
	}
	ws := &weiboHTTPSession{cookies: map[string]string{}}
	q := url.Values{"entry": {"miniblog"}, "size": {"180"}}
	res, err := weiboHTTPGet(ctx, env, ws, weiboHTTPBase+weiboQRImagePath+"?"+q.Encode())
	if err != nil {
		return "", "", err
	}
	var envBody weiboEnvelope
	ok, err := login.DecodeJSONP(res.Body, &envBody)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", errors.New("weibo 扫码: 二维码响应为空")
	}
	if envBody.Retcode != weiboRetSuccess || envBody.Data.QRID == "" || envBody.Data.Image == "" {
		return "", "", errors.New("weibo 扫码: " + login.Fallback(envBody.Msg, "取码失败"))
	}
	ws.qrid = envBody.Data.QRID
	sess.Data = ws
	qr := env.FetchImage(ctx, strings.ReplaceAll(envBody.Data.Image, `\/`, "/"), weiboHTTPBase+"/")
	if qr == "" {
		return "", "", errors.New("weibo 扫码: 二维码图片下载失败")
	}
	return qr, weiboScanText(envBody.Data.Image, envBody.Data.QRID), nil
}

// weiboScanText 二维码图片 URL 的 data 参数就是扫码链接；退化时按 qrid 拼。
func weiboScanText(image, qrid string) string {
	if u, err := url.Parse(strings.ReplaceAll(image, `\/`, "/")); err == nil {
		if s := u.Query().Get("data"); s != "" {
			return s
		}
	}
	return "https://passport.weibo.cn/signin/qrcode/scan?qr=" + url.QueryEscape(qrid)
}

func weiboHTTPPoll(ctx context.Context, env *login.Env, sess *login.Session) (bool, map[string]string, string, error) {
	ws, _ := sess.Data.(*weiboHTTPSession)
	if ws == nil || ws.qrid == "" {
		return false, nil, "", errors.New("weibo 登录会话缺失")
	}
	if time.Since(ws.lastPoll) < weiboPollGap {
		return false, nil, "", nil
	}
	ws.lastPoll = time.Now()
	q := url.Values{
		"entry": {"miniblog"}, "source": {"miniblog"}, "url": {"https://weibo.com/"},
		"qrid": {ws.qrid}, "rid": {""}, "ver": {"20250520"},
	}
	res, err := weiboHTTPGet(ctx, env, ws, weiboHTTPBase+weiboQRCheckPath+"?"+q.Encode())
	if err != nil {
		return false, nil, "", err
	}
	var envBody weiboEnvelope
	if _, err := login.DecodeJSONP(res.Body, &envBody); err != nil {
		return false, nil, "", err
	}
	switch envBody.Retcode {
	case weiboRetSuccess:
		var finishErr error
		if envBody.Data.URL != "" {
			finishErr = weiboFinish(ctx, env, ws, envBody.Data.URL)
		}
		cookies := snapshotCookies(ws.cookies)
		if !weiboLoggedIn(cookies) && envBody.Data.Alt != "" {
			finishErr = weiboFinishAlt(ctx, env, ws, envBody.Data.Alt)
			cookies = snapshotCookies(ws.cookies)
		}
		if !weiboLoggedIn(cookies) {
			if finishErr != nil {
				return false, nil, "", finishErr
			}
			return false, nil, "", errors.New("weibo 扫码成功但未取到登录 cookie")
		}
		env.InjectCookies(ctx, ".weibo.com", cookies)
		return true, cookies, weiboUser(cookies), nil
	case weiboRetScanned:
		return false, nil, "", nil
	case weiboRetExpired1, weiboRetExpired2:
		return false, nil, "", login.ErrExpired
	default:
		return false, nil, "", nil
	}
}

func snapshotCookies(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for name, value := range src {
		dst[name] = value
	}
	return dst
}

// weiboFinish 访问新版 check 给的一次性换票链接，并跟进页面里的跨域 ticket。
func weiboFinish(ctx context.Context, env *login.Env, ws *weiboHTTPSession, rawURL string) error {
	target := strings.ReplaceAll(rawURL, `\/`, "/")
	if target == "" {
		return nil
	}
	res, err := weiboHTTPGetFollow(ctx, env, ws, target)
	if err != nil {
		return err
	}
	for _, u := range login.TicketURLs(res.Body) {
		_, _ = weiboHTTPGet(ctx, env, ws, u)
	}
	return nil
}

// weiboFinishAlt 旧版 check 只给 alt：先换 ticket 列表，再逐个访问跨域链接。
func weiboFinishAlt(ctx context.Context, env *login.Env, ws *weiboHTTPSession, alt string) error {
	if alt == "" {
		return errors.New("weibo 扫码成功但缺少换票地址")
	}
	target := "https://login.sina.com.cn/sso/login.php?entry=miniblog&crossdomain=1&gateway=1&savestate=30&alt=" +
		url.QueryEscape(alt)
	res, err := weiboHTTPGetFollow(ctx, env, ws, target)
	if err != nil {
		return err
	}
	for _, u := range login.TicketURLs(res.Body) {
		_, _ = weiboHTTPGet(ctx, env, ws, u)
	}
	return nil
}
