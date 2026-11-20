// 载具 profile 的 cookie 读写。浏览器级 CDP 连接(无 sessionId)上实测 WebView2
// 只支持 Storage.getCookies / Storage.setCookies:Network.getAllCookies 与
// Network.deleteCookies 是 page session 才有的方法。删除用 setCookies 把过期
// 时间置到过去,避免 Storage.clearCookies 清空整库。

package browser

import (
	"context"
	"strings"
	"time"

	"mygo-host/platform"
)

const cookieCmdTimeout = 15 * time.Second

// AllCookies 读整个 profile 的 cookie(含 HttpOnly);载具未运行时会被拉起。
func (b *Bridge) AllCookies(ctx context.Context) ([]platform.BrowserCookie, error) {
	conn, err := b.ensureConn(ctx)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, cookieCmdTimeout)
	defer cancel()
	var out struct {
		Cookies []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
			Path   string `json:"path"`
		} `json:"cookies"`
	}
	if err := conn.call(cctx, "Storage.getCookies", nil, "", &out); err != nil {
		return nil, err
	}
	cookies := make([]platform.BrowserCookie, 0, len(out.Cookies))
	for _, c := range out.Cookies {
		cookies = append(cookies, platform.BrowserCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
		})
	}
	return cookies, nil
}

// ClearCookies 删除域名落在 hosts(自身或子域)下的全部 cookie,返回删除条数。
func (b *Bridge) ClearCookies(ctx context.Context, hosts []string) (int, error) {
	all, err := b.AllCookies(ctx)
	if err != nil {
		return 0, err
	}
	var targets []platform.BrowserCookie
	for _, c := range all {
		if cookieDomainMatch(c.Domain, hosts) {
			targets = append(targets, c)
		}
	}
	if len(targets) == 0 {
		return 0, nil
	}
	conn, err := b.ensureConn(ctx)
	if err != nil {
		return 0, err
	}
	cctx, cancel := context.WithTimeout(ctx, cookieCmdTimeout)
	defer cancel()
	cleared := 0
	for _, c := range targets {
		path := c.Path
		if path == "" {
			path = "/"
		}
		cookie := map[string]any{
			"name": c.Name, "value": c.Value, "domain": c.Domain,
			"path": path, "expires": 1, // 1970-01-01,等价于删除
		}
		if err := conn.call(cctx, "Storage.setCookies", map[string]any{"cookies": []any{cookie}}, "", nil); err != nil {
			return cleared, err
		}
		cleared++
	}
	return cleared, nil
}

// injectedCookieTTL 回灌 cookie 的持久时间:会话 cookie 在载具重启后会丢,
// 登录态必须显式给过期时间。
const injectedCookieTTL = 180 * 24 * time.Hour

// SetCookies 把 cookie 写进 profile(纯 HTTP 扫码登录成功后回灌浏览器)。
func (b *Bridge) SetCookies(ctx context.Context, cookies []platform.BrowserCookie) error {
	if len(cookies) == 0 {
		return nil
	}
	conn, err := b.ensureConn(ctx)
	if err != nil {
		return err
	}
	expires := float64(time.Now().Add(injectedCookieTTL).Unix())
	payload := make([]any, 0, len(cookies))
	for _, c := range cookies {
		path := c.Path
		if path == "" {
			path = "/"
		}
		payload = append(payload, map[string]any{
			"name": c.Name, "value": c.Value, "domain": c.Domain,
			"path": path, "expires": expires,
		})
	}
	cctx, cancel := context.WithTimeout(ctx, cookieCmdTimeout)
	defer cancel()
	return conn.call(cctx, "Storage.setCookies", map[string]any{"cookies": payload}, "", nil)
}

// cookieDomainMatch 按标签边界匹配:notxiaohongshu.com 不算 xiaohongshu.com。
func cookieDomainMatch(domain string, hosts []string) bool {
	d := strings.ToLower(strings.TrimPrefix(domain, "."))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimPrefix(h, "."))
		if h == "" {
			continue
		}
		if d == h || strings.HasSuffix(d, "."+h) {
			return true
		}
	}
	return false
}
