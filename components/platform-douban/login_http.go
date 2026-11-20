// 豆瓣扫码的纯 HTTP 会话：generate 拿 login_url/img，cookie 全程由本会话
// 维持；轮询到 login 后，从 Set-Cookie 与豆瓣域 cookie 中取登录态。

package douban

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const (
	doubanHTTPBase     = "https://accounts.douban.com"
	doubanQRCodePath   = "/j/mobile/login/qrlogin_code"
	doubanQRStatusPath = "/j/mobile/login/qrlogin_status"
	doubanLoginOKURL   = "https://www.douban.com/stat.html?&action=login_success" +
		"&platform=qrcode&callback=jsonp_mygo"
	doubanHTTPPollGap = 2 * time.Second
)

type doubanHTTPSession struct {
	code     string
	cookies  map[string]string
	lastPoll time.Time
}

type doubanEnvelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Payload struct {
		Code        string `json:"code"`
		LoginURL    string `json:"login_url"`
		Img         string `json:"img"`
		LoginStatus string `json:"login_status"`
	} `json:"payload"`
}

func doubanHeaders(cookies map[string]string) map[string]string {
	h := map[string]string{
		"referer": doubanHTTPBase + "/passport/login", "origin": doubanHTTPBase,
		"accept": "application/json", "x-requested-with": "XMLHttpRequest",
	}
	if ck := login.CookieHeader(cookies); ck != "" {
		h["cookie"] = ck
	}
	return h
}

func doubanDo(ctx context.Context, env *login.Env, sess *doubanHTTPSession, method, path, body string) (*platform.Response, *doubanEnvelope, error) {
	headers := doubanHeaders(sess.cookies)
	if body != "" {
		headers["content-type"] = "application/x-www-form-urlencoded"
	}
	res, err := env.HTTP.Do(ctx, platform.Request{
		URL: doubanHTTPBase + path, Method: method, Headers: headers, Body: body,
	})
	if err != nil {
		return nil, nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, nil, errors.New("douban 扫码: HTTP " + strconv.Itoa(res.Status))
	}
	login.MergeSetCookies(sess.cookies, res.Headers["set-cookie"])
	var envBody doubanEnvelope
	if err := json.Unmarshal([]byte(res.Body), &envBody); err != nil {
		return res, nil, err
	}
	if envBody.Status != "success" {
		return res, &envBody, errors.New("douban 扫码: " + login.Fallback(envBody.Message, envBody.Status))
	}
	return res, &envBody, nil
}

// doubanHTTPStart 直接申请二维码；qrlogin_code 自己会下发 bid 会话 cookie。
func doubanHTTPStart(ctx context.Context, env *login.Env, sess *login.Session) (string, string, error) {
	if env.HTTP == nil {
		return "", "", errors.New("douban 扫码: 缺少 HTTP 客户端")
	}
	ds := &doubanHTTPSession{cookies: map[string]string{}}
	genCtx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	_, envBody, err := doubanDo(genCtx, env, ds, "GET", doubanQRCodePath, "")
	if err != nil {
		return "", "", err
	}
	ds.code = envBody.Payload.Code
	if ds.code == "" {
		return "", "", errors.New("douban 扫码: 二维码 code 为空")
	}
	sess.Data = ds
	// 空图会让前端显示空白框；明确报错，交给上层决定是否回退浏览器。
	qr := env.FetchImage(ctx, envBody.Payload.Img, doubanHTTPBase+"/passport/login")
	if qr == "" {
		return "", "", errors.New("douban 扫码: 二维码图片下载失败")
	}
	return qr, envBody.Payload.LoginURL, nil
}

// doubanHTTPPoll 轮询扫码状态；login 后从会话 cookie 取 dbcl2。
func doubanHTTPPoll(ctx context.Context, env *login.Env, sess *login.Session) (bool, map[string]string, string, error) {
	ds, _ := sess.Data.(*doubanHTTPSession)
	if ds == nil || ds.code == "" {
		return false, nil, "", errors.New("douban 登录会话缺失")
	}
	if time.Since(ds.lastPoll) < doubanHTTPPollGap {
		return false, nil, "", nil
	}
	ds.lastPoll = time.Now()
	pctx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	_, envBody, err := doubanDo(pctx, env, ds, "GET",
		doubanQRStatusPath+"?ck=&code="+url.QueryEscape(ds.code), "")
	if err != nil {
		return false, nil, "", err
	}
	switch strings.ToLower(envBody.Payload.LoginStatus) {
	case "login":
		doubanFinish(ctx, env, ds)
		cookies := map[string]string{}
		for name, value := range ds.cookies {
			cookies[name] = value
		}
		if cookies["dbcl2"] == "" {
			return false, nil, "", errors.New("douban 扫码成功但未取到 dbcl2,请重新扫码")
		}
		env.InjectCookies(ctx, ".douban.com", cookies)
		return true, cookies, doubanUser(cookies), nil
	case "invalid", "cancel", "expired":
		return false, nil, "", login.ErrExpired
	}
	return false, nil, "", nil
}

// doubanFinish 扫码确认后补访登录成功页：部分地区/节点只在此时才把
// dbcl2 写进 Set-Cookie。失败不阻断主流程，后续仍按会话 cookie 校验。
func doubanFinish(ctx context.Context, env *login.Env, ds *doubanHTTPSession) {
	fctx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	res, err := env.HTTP.Do(fctx, platform.Request{
		URL: doubanLoginOKURL, Headers: doubanHeaders(ds.cookies),
	})
	if err == nil {
		login.MergeSetCookies(ds.cookies, res.Headers["set-cookie"])
	}
	if ds.cookies["dbcl2"] != "" {
		return
	}
	res, err = env.HTTP.Do(fctx, platform.Request{
		URL: doubanHTTPBase + "/", Headers: doubanHeaders(ds.cookies),
	})
	if err == nil {
		login.MergeSetCookies(ds.cookies, res.Headers["set-cookie"])
	}
}
