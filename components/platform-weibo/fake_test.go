// 测试替身:内存 Browser/BrowserPage(记录调用序列、返回 canned 捕获体),
// 不触达真实载具与外网。

package weibo

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

// fakeBrowser 记录 Do/CloseTab 调用;Do 直接把 fakePage 交给回调。
type fakeBrowser struct {
	calls      []string
	responses  []platform.CapturedResponse
	captureErr error
	doErr      error
	closeErr   error

	origin     string
	captureURL string
	match      func(*url.URL) bool
	idle       time.Duration
	timeout    time.Duration
	closed     int
}

func (f *fakeBrowser) Do(ctx context.Context, origin string, fn func(platform.BrowserPage) error) error {
	f.calls = append(f.calls, "Do:"+origin)
	f.origin = origin
	if f.doErr != nil {
		return f.doErr
	}
	return fn(&fakePage{browser: f})
}

func (f *fakeBrowser) CloseTab(ctx context.Context, origin string) error {
	f.calls = append(f.calls, "CloseTab:"+origin)
	f.closed++
	return f.closeErr
}

type fakePage struct{ browser *fakeBrowser }

func (p *fakePage) CaptureNavigate(ctx context.Context, rawURL string, match func(*url.URL) bool,
	idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	p.browser.calls = append(p.browser.calls, "CaptureNavigate:"+rawURL)
	p.browser.captureURL = rawURL
	p.browser.match = match
	p.browser.idle = idle
	p.browser.timeout = timeout
	return p.browser.responses, p.browser.captureErr
}

func (p *fakePage) Capture(context.Context, func() error, func(*url.URL) bool,
	time.Duration, time.Duration) ([]platform.CapturedResponse, error) {
	return nil, errors.New("fake: 不应调用 Capture")
}

func (p *fakePage) Navigate(context.Context, string) error {
	return errors.New("fake: 不应直接 Navigate")
}

func (p *fakePage) Evaluate(context.Context, string) (any, error) {
	return nil, errors.New("fake: 不应直接 Evaluate")
}

func (p *fakePage) WaitUntil(context.Context, string, time.Duration) error {
	return errors.New("fake: 不应直接 WaitUntil")
}

func (p *fakePage) Cookies(context.Context, string) (map[string]string, error) {
	return nil, errors.New("fake: 不应读取 cookie")
}

var (
	_ platform.Browser     = (*fakeBrowser)(nil)
	_ platform.BrowserPage = (*fakePage)(nil)
)

// testContext 固定 Now,便于断言 fetchedAt 与 URL 模板。
func testContext(b *fakeBrowser) *platform.ServiceContext {
	return &platform.ServiceContext{Browser: b, Now: func() int64 { return testNow }}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// capturedURL 构造页面自发 getIndex 的 URL(带 visitor 流程追加的参数)。
func capturedURL(container string) string {
	return "https://m.weibo.cn/api/container/getIndex?containerid=" +
		url.QueryEscape(container) + "&display=0&retcode=6102"
}

func userContainer(keyword string) string {
	return "100103type=3&q=" + keyword
}

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
