// 测试替身:内存 Browser/BrowserPage(记录调用序列并按表达式返回 canned 状态),
// 不触达真实载具与外网。

package xhs

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"mygo-host/platform"
)

const testNow = int64(1700000000000)

// fakeBrowser 记录 Do/CloseTab 调用;Do 直接把 fakePage 交给回调。
type fakeBrowser struct {
	page     *fakePage
	calls    []string
	doErr    error
	closeErr error
	closed   int
}

func newFakeBrowser() (*fakeBrowser, *fakePage) {
	page := &fakePage{}
	browser := &fakeBrowser{page: page}
	page.browser = browser
	return browser, page
}

func (b *fakeBrowser) Do(ctx context.Context, origin string, fn func(platform.BrowserPage) error) error {
	b.calls = append(b.calls, "Do:"+origin)
	if b.doErr != nil {
		return b.doErr
	}
	return fn(b.page)
}

func (b *fakeBrowser) CloseTab(ctx context.Context, origin string) error {
	b.calls = append(b.calls, "CloseTab:"+origin)
	b.closed++
	return b.closeErr
}

// fakePage 按表达式返回登录/风控/state 的 canned 值,并记录两段捕获参数。
type fakePage struct {
	browser *fakeBrowser

	risk       bool
	loginPage  bool
	pageLogged any // true / false / nil(读不到)
	domLogged  bool
	cookies    map[string]string
	cookieErr  error
	state      any // SSR 投影原始返回值
	clickErr   error
	evalErrs   map[string]error

	navCaptureURL string
	navMatch      func(*url.URL) bool
	navIdle       time.Duration
	navTimeout    time.Duration
	navResponses  []platform.CapturedResponse
	navErr        error

	clickMatch      func(*url.URL) bool
	clickIdle       time.Duration
	clickTimeout    time.Duration
	clickResponses  []platform.CapturedResponse
	clickCaptureErr error
	clicked         bool
}

func (p *fakePage) record(label string) { p.browser.calls = append(p.browser.calls, label) }

func (p *fakePage) Navigate(context.Context, string) error {
	p.record("Navigate")
	return nil
}

func (p *fakePage) WaitUntil(context.Context, string, time.Duration) error {
	p.record("WaitUntil")
	return nil
}

func (p *fakePage) Evaluate(ctx context.Context, expression string) (any, error) {
	label := expressionLabel(expression)
	p.record(label)
	if err := p.evalErrs[label]; err != nil {
		return nil, err
	}
	switch expression {
	case riskBoxExpr:
		return p.risk, nil
	case loginPageExpr:
		return p.loginPage, nil
	case pageLoggedInExpr:
		return p.pageLogged, nil
	case domLoggedInExpr:
		return p.domLogged, nil
	case xhsPageLoggedExpr:
		return p.pageLogged, nil
	case xhsDomLoggedExpr:
		return p.domLogged, nil
	case ssrUsersExpr:
		return p.state, nil
	case clickUserTabExpr:
		p.clicked = true
		return true, p.clickErr
	}
	return nil, fmt.Errorf("fake: 未预期的表达式 %q", expression)
}

func (p *fakePage) Cookies(ctx context.Context, rawURL string) (map[string]string, error) {
	p.record("Cookies")
	if p.cookieErr != nil {
		return nil, p.cookieErr
	}
	return p.cookies, nil
}

func (p *fakePage) CaptureNavigate(ctx context.Context, rawURL string, match func(*url.URL) bool,
	idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	p.record("CaptureNavigate")
	p.navCaptureURL, p.navMatch = rawURL, match
	p.navIdle, p.navTimeout = idle, timeout
	return p.navResponses, p.navErr
}

func (p *fakePage) Capture(ctx context.Context, trigger func() error, match func(*url.URL) bool,
	idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	p.record("Capture")
	p.clickMatch = match
	p.clickIdle, p.clickTimeout = idle, timeout
	if err := trigger(); err != nil {
		return p.clickResponses, err
	}
	return p.clickResponses, p.clickCaptureErr
}

func expressionLabel(expression string) string {
	switch expression {
	case riskBoxExpr:
		return "Risk"
	case loginPageExpr:
		return "LoginPage"
	case pageLoggedInExpr:
		return "PageLoggedIn"
	case domLoggedInExpr:
		return "DOM"
	case xhsPageLoggedExpr:
		return "LoginPageLogged"
	case xhsDomLoggedExpr:
		return "LoginDOM"
	case ssrUsersExpr:
		return "State"
	case clickUserTabExpr:
		return "Click"
	}
	return "Unknown"
}

var (
	_ platform.Browser     = (*fakeBrowser)(nil)
	_ platform.BrowserPage = (*fakePage)(nil)
)

func testContext(browser platform.Browser) *platform.ServiceContext {
	return &platform.ServiceContext{Browser: browser, Now: func() int64 { return testNow }}
}

// assertCalls 严格断言完整调用序列。
func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q(全量 %v)", i, got[i], want[i], got)
		}
	}
}

// assertOrder 子序列断言(允许中间夹其它调用)。
func assertOrder(t *testing.T, calls []string, want ...string) {
	t.Helper()
	index := 0
	for _, call := range calls {
		if index < len(want) && call == want[index] {
			index++
		}
	}
	if index != len(want) {
		t.Fatalf("调用顺序缺少 %q(全量 %v)", want[index:], calls)
	}
}

func assertNoCall(t *testing.T, calls []string, forbidden ...string) {
	t.Helper()
	for _, call := range calls {
		for _, bad := range forbidden {
			if call == bad {
				t.Fatalf("出现不应有的调用 %q(全量 %v)", bad, calls)
			}
		}
	}
}
