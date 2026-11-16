// 测试夹具：fake RoundTripper 拦截全部 HTTP（离线，不联网），记录 URL/Header；
// ServiceContext 注入固定时钟（与 TS 自检的 appHost.now 同法）。

package bili

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mygo-host/platform"
)

const testNow = int64(1_700_000_000_123)

type recordedRequest struct {
	URL    string
	Header http.Header
}

type fakeServer struct {
	mu       sync.Mutex
	requests []recordedRequest
	respond  func(*http.Request) (*http.Response, error)
}

func (f *fakeServer) roundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{URL: req.URL.String(), Header: req.Header.Clone()})
	respond := f.respond
	f.mu.Unlock()
	if respond != nil {
		return respond(req)
	}
	return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
}

// find 最近一次 URL 含 substr 的请求。
func (f *fakeServer) find(substr string) *recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		if strings.Contains(f.requests[i].URL, substr) {
			req := f.requests[i]
			return &req
		}
	}
	return nil
}

func (f *fakeServer) reset() {
	f.mu.Lock()
	f.requests = nil
	f.mu.Unlock()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func cookieResponse(status int, body string, cookies []string) *http.Response {
	res := jsonResponse(status, body)
	if len(cookies) > 0 {
		res.Header["Set-Cookie"] = cookies
	}
	return res
}

func newTestContext(t *testing.T, f *fakeServer, now int64) *platform.ServiceContext {
	t.Helper()
	return &platform.ServiceContext{
		Logf:  func(format string, args ...any) { t.Logf(format, args...) },
		HTTP:  &platform.HostClient{HTTP: &http.Client{Transport: roundTripFunc(f.roundTrip)}},
		Creds: platform.NewCredentials(filepath.Join(t.TempDir(), "credentials.json")),
		Now:   func() int64 { return now },
	}
}

// navResponder nav 返回固定 wbi_img（其余 URL 交给 next）。
func navResponder(imgKey, subKey string, next func(*http.Request) (*http.Response, error)) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/web-interface/nav") {
			body := fmt.Sprintf(`{"code":0,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/%s.png","sub_url":"https://i0.hdslb.com/bfs/wbi/%s.png"}}}`, imgKey, subKey)
			return jsonResponse(http.StatusOK, body), nil
		}
		if next != nil {
			return next(req)
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
	}
}

func mustContain(t *testing.T, haystack, needle, label string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("%s: %q 不含 %q", label, haystack, needle)
	}
}
