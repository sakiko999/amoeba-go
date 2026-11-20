// 微博登录配方：纯 HTTP 优先，失败回退浏览器登录页。匿名访客也会种
// SUB+SUBP，登录判定必须看 SSOLoginState/ALF/SUL 或 MLOGIN=1。

package weibo

import (
	"net/url"
	"time"

	"mygo-host/platform/login"
)

// LoginFlow 返回微博登录配方。
func LoginFlow() login.Flow {
	return login.Flow{
		ID: "weibo", Label: label, Color: "#e6162d", Hosts: []string{"weibo.com", "weibo.cn"},
		Origin: "https://passport.weibo.com", LoginCookie: "SUB",
		Timeout: 300 * time.Second, TrustCookies: true,
		Check: weiboLoggedIn, HTTPStart: weiboHTTPStart, HTTPPoll: weiboHTTPPoll,
		Open: weiboOpen, UserHint: weiboUser,
	}
}

func weiboLoggedIn(cookies map[string]string) bool {
	if cookies["SUB"] == "" {
		return false
	}
	return cookies["SSOLoginState"] != "" || cookies["ALF"] != "" ||
		cookies["SUL"] != "" || cookies["MLOGIN"] == "1"
}

func weiboUser(cookies map[string]string) string {
	uid := cookies["SUBP"]
	if uid == "" {
		return ""
	}
	if decoded, err := url.QueryUnescape(uid); err == nil {
		return decoded
	}
	return uid
}
