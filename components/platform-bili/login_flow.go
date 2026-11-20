// B 站扫码登录：复用本组件的 generate/poll 协议，不打开浏览器。

package bili

import (
	"context"
	"errors"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

const (
	biliCookieDomain = ".bilibili.com"
	biliPollGap      = 2 * time.Second
)

type biliSession struct {
	key      string
	lastPoll time.Time
}

// LoginFlow 返回 B 站登录配方。
func LoginFlow() login.Flow {
	return login.Flow{
		ID: "bili", Label: "B站", Color: "#fb7299", Hosts: []string{"bilibili.com"},
		Origin: "https://www.bilibili.com", LoginCookie: "SESSDATA",
		Timeout: 180 * time.Second, TrustCookies: true,
		HTTPStart: biliHTTPStart, HTTPPoll: biliHTTPPoll, UserHint: loginBiliUser,
	}
}

func biliHTTPStart(ctx context.Context, env *login.Env, sess *login.Session) (string, string, error) {
	cctx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	qr, err := QRGenerate(cctx, &platform.ServiceContext{HTTP: env.HTTP})
	if err != nil {
		return "", "", err
	}
	sess.Data = &biliSession{key: qr.Key}
	return "", qr.URL, nil
}

func biliHTTPPoll(ctx context.Context, env *login.Env, sess *login.Session) (bool, map[string]string, string, error) {
	bs, _ := sess.Data.(*biliSession)
	if bs == nil || bs.key == "" {
		return false, nil, "", errors.New("bili 登录会话缺失")
	}
	if time.Since(bs.lastPoll) < biliPollGap {
		return false, nil, "", nil
	}
	bs.lastPoll = time.Now()
	cctx, cancel := context.WithTimeout(ctx, login.HTTPTimeout)
	defer cancel()
	poll, err := QRPoll(cctx, &platform.ServiceContext{HTTP: env.HTTP}, bs.key)
	if err != nil {
		return false, nil, "", err
	}
	switch poll.Status {
	case QRStatusExpired:
		return false, nil, "", login.ErrExpired
	case QRStatusSuccess:
		cookies := login.ParseCookiePairs(poll.Cookie)
		if cookies["SESSDATA"] == "" {
			return false, nil, "", errors.New("bili 扫码成功但未取到 SESSDATA")
		}
		env.InjectCookies(ctx, biliCookieDomain, cookies)
		return true, cookies, cookies["DedeUserID"], nil
	}
	return false, nil, "", nil
}

func loginBiliUser(cookies map[string]string) string { return cookies["DedeUserID"] }
