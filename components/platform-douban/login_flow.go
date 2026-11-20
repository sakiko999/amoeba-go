// 豆瓣登录配方：HTTP 优先（qrlogin_code/status，成功后 Set-Cookie 种
// dbcl2），失败再回退浏览器登录页。

package douban

import (
	"time"

	"mygo-host/platform/login"
)

// LoginFlow 返回豆瓣登录配方。
func LoginFlow() login.Flow {
	return login.Flow{
		ID: "douban", Label: label, Color: "#2e963d", Hosts: []string{"douban.com"},
		Origin: "https://accounts.douban.com", LoginCookie: "dbcl2",
		Timeout: 300 * time.Second, TrustCookies: true,
		HTTPStart: doubanHTTPStart, HTTPPoll: doubanHTTPPoll,
		Open: doubanOpen, UserHint: doubanUser,
	}
}
