// 扫码登录测试替身：把平台固定 URL 重写到本地 httptest，并提供登录侧
// Browser 的记录实现。供 platform-*、login 与 service-credentials 的测试使用。

package logintest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

// RewriteTransport 把任意请求改写到测试服务器（保留原 path 与 Host 头）。
type RewriteTransport struct{ Target *url.URL }

func (t RewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.Target.Scheme
	clone.URL.Host = t.Target.Host
	clone.Host = t.Target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

// Env 建一套重写到 h 的登录环境；调用方负责 Close 返回的 server。
func Env(h http.Handler, b login.Browser) (*login.Env, *httptest.Server) {
	srv := httptest.NewServer(h)
	target, _ := url.Parse(srv.URL)
	return &login.Env{
		HTTP:    &platform.HostClient{HTTP: &http.Client{Transport: RewriteTransport{Target: target}}},
		Browser: b,
	}, srv
}

// Browser 记录登录侧调用；Page 非 nil 时 DoAs 直接把页面交给回调。
type Browser struct {
	Page      platform.BrowserPage
	Cookies   []platform.BrowserCookie
	Injected  []platform.BrowserCookie
	Held      []string
	DoAsCalls int
	CloseAs   int
	CookieAs  int
	Cleared   [][]string
}

func (b *Browser) Hold(_ context.Context, key, _ string) error {
	b.Held = append(b.Held, key)
	return nil
}

func (b *Browser) DoAs(_ context.Context, _ string, _ string, fn func(platform.BrowserPage) error) error {
	b.DoAsCalls++
	if b.Page == nil {
		return errors.New("logintest: 未提供 Page")
	}
	return fn(b.Page)
}

func (b *Browser) CloseTabAs(context.Context, string) error {
	b.CloseAs++
	return nil
}

func (b *Browser) CookiesAs(context.Context, string, string) ([]platform.BrowserCookie, error) {
	b.CookieAs++
	return b.Cookies, nil
}

func (b *Browser) ClearCookies(_ context.Context, hosts []string) (int, error) {
	b.Cleared = append(b.Cleared, hosts)
	return len(b.Cookies), nil
}

func (b *Browser) SetCookies(_ context.Context, cookies []platform.BrowserCookie) error {
	b.Injected = append(b.Injected, cookies...)
	return nil
}

// Page 是最小 BrowserPage 替身；需要真实页面逻辑的测试自行提供实现。
type Page struct{}

func (p *Page) Navigate(context.Context, string) error { return nil }

func (p *Page) Evaluate(context.Context, string) (any, error) { return nil, nil }

func (p *Page) WaitUntil(context.Context, string, time.Duration) error { return nil }

func (p *Page) Cookies(context.Context, string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (p *Page) CaptureNavigate(context.Context, string, func(*url.URL) bool,
	time.Duration, time.Duration) ([]platform.CapturedResponse, error) {
	return nil, nil
}

func (p *Page) Capture(context.Context, func() error, func(*url.URL) bool,
	time.Duration, time.Duration) ([]platform.CapturedResponse, error) {
	return nil, nil
}
