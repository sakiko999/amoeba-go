// 测试共用:固定时钟上下文、包级状态重置、fixture 读取与假浏览器。
// 所有测试同包串行执行(会改包级 URL/缓存变量,禁止 t.Parallel)。

package douyin

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"mygo-host/platform"
)

const testNow = int64(1700000000000)

func testContext() *platform.ServiceContext {
	return &platform.ServiceContext{
		HTTP: platform.NewHostClient(),
		JS: platform.JSRunnerFunc(func(context.Context, string, string, []any) (any, error) {
			return "ab/c=", nil
		}),
		Now: func() int64 { return testNow },
	}
}

// resetState 清包级可变状态(用户缓存/匿名 cookie)。
func resetState() {
	userCache = userCacheState{m: map[string]userHit{}}
	anonCookie = cookieJarState{}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type jsCall struct {
	code string
	fn   string
	args []any
}

func captureJS(target *jsCall) platform.JSRunner {
	return platform.JSRunnerFunc(func(_ context.Context, code, fn string, args []any) (any, error) {
		target.code, target.fn, target.args = code, fn, args
		return "ab/c=", nil
	})
}

type fakeBrowser struct {
	cookies      map[string]string
	cookiesErr   error
	captured     []platform.CapturedResponse
	captureErr   error
	origin       string
	cookiesURL   string
	captureURL   string
	match        func(*url.URL) bool
	idle         time.Duration
	timeout      time.Duration
	captureCalls int
	closeCalls   int
	closeCtxErr  error
}

func (b *fakeBrowser) Do(_ context.Context, origin string, fn func(platform.BrowserPage) error) error {
	b.origin = origin
	return fn(&fakePage{browser: b})
}

func (b *fakeBrowser) CloseTab(ctx context.Context, _ string) error {
	b.closeCalls++
	b.closeCtxErr = ctx.Err()
	return ctx.Err()
}

type fakePage struct{ browser *fakeBrowser }

func (p *fakePage) Navigate(context.Context, string) error { return errUnexpectedCall }

func (p *fakePage) Evaluate(context.Context, string) (any, error) { return nil, errUnexpectedCall }

func (p *fakePage) WaitUntil(context.Context, string, time.Duration) error {
	return errUnexpectedCall
}

func (p *fakePage) Cookies(_ context.Context, rawURL string) (map[string]string, error) {
	p.browser.cookiesURL = rawURL
	return p.browser.cookies, p.browser.cookiesErr
}

func (p *fakePage) CaptureNavigate(_ context.Context, rawURL string, match func(*url.URL) bool, idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	p.browser.captureCalls++
	p.browser.captureURL, p.browser.match = rawURL, match
	p.browser.idle, p.browser.timeout = idle, timeout
	return p.browser.captured, p.browser.captureErr
}

func (p *fakePage) Capture(context.Context, func() error, func(*url.URL) bool,
	time.Duration, time.Duration) ([]platform.CapturedResponse, error) {
	return nil, errUnexpectedCall
}

var errUnexpectedCall = errors.New("fakeBrowser: 未预期的页面调用")
