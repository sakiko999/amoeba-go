// CaptureNavigate 单测:完成顺序、MIME 过滤、重定向最后一跳、base64 解码、
// 「有请求永不结束 → 超时返回部分、不报错」,以及默认拦截表的图片边界。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestCaptureNavigate(t *testing.T) {
	f := newFakeCDP(t)
	rec := &bodyRecorder{}
	net := &netRecorder{}
	installCaptureEval(f)
	installCaptureBodies(f, rec)
	installNetworkRecorder(f, net)
	base := f.srv.URL
	emitCaptureTraffic(f, base)

	b := newFakeBridge(t, f)
	var got []platform.CapturedResponse
	err := b.Do(context.Background(), base, func(p platform.BrowserPage) error {
		var err error
		got, err = p.CaptureNavigate(context.Background(), base+"/page",
			func(u *url.URL) bool { return strings.HasPrefix(u.Path, "/api/") },
			150*time.Millisecond, 600*time.Millisecond)
		return err
	})
	if err != nil {
		t.Fatalf("超时不报错, got %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("captured = %d, want 3: %+v", len(got), got)
	}
	if got[0].URL != base+"/api/users?q=x" || got[0].Body != `{"ok":true}` {
		t.Fatalf("captured[0] = %+v", got[0])
	}
	if got[1].URL != base+"/api/final" || got[1].Body != "hello" {
		t.Fatalf("captured[1] = %+v(重定向最后一跳 + base64 解码)", got[1])
	}
	if got[2].URL != base+"/api/nomime" || got[2].Body != "nomime" {
		t.Fatalf("captured[2] = %+v(空 mime 应保留)", got[2])
	}
	if ids := rec.list(); len(ids) != 3 || ids[0] != "r1" || ids[1] != "r3" || ids[2] != "r5" {
		t.Fatalf("getResponseBody 请求 = %v, want [r1 r3 r5](r2 非文本、r4 未完成、r6 其他 session)", ids)
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if net.enable["maxTotalBufferSize"] != 67108864 || net.enable["maxResourceBufferSize"] != 16777216 {
		t.Fatalf("Network.enable 参数 = %v", net.enable)
	}
	if net.disables != 0 {
		t.Fatalf("Network.disable 次数 = %d, want 0(默认 Network 不随捕获关闭)", net.disables)
	}
	if len(net.blocked) != len(blockedURLPatterns) {
		t.Fatalf("Network.setBlockedURLs = %d 条, want %d", len(net.blocked), len(blockedURLPatterns))
	}
}

// 默认拦截属于 target 生命周期:普通 Do 也装拦截表,捕获结束不关闭 Network。
func TestDefaultResourceBlockingLifecycle(t *testing.T) {
	f := newFakeCDP(t)
	net := &netRecorder{}
	installNetworkRecorder(f, net)
	installCaptureEval(f)
	installCaptureBodies(f, &bodyRecorder{})
	f.handle("Page.navigate", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"frameId": "f1", "loaderId": "l1"})
	})

	b := newFakeBridge(t, f)
	ctx := context.Background()
	err := b.Do(ctx, "http://default-block.test", func(platform.BrowserPage) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	net.mu.Lock()
	defaultBlocked := len(net.blocked)
	net.mu.Unlock()
	if defaultBlocked != len(blockedURLPatterns) {
		t.Fatalf("默认拦截表 = %d 条, want %d", defaultBlocked, len(blockedURLPatterns))
	}

	err = b.Do(ctx, "http://default-block.test", func(p platform.BrowserPage) error {
		_, err := p.Capture(ctx, func() error { return nil }, func(*url.URL) bool { return false },
			50*time.Millisecond, 500*time.Millisecond)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if net.disables != 0 {
		t.Fatalf("捕获后 Network.disable = %d, want 0", net.disables)
	}
	if len(net.blocked) != len(blockedURLPatterns) {
		t.Fatalf("捕获后拦截表 = %d 条, want %d", len(net.blocked), len(blockedURLPatterns))
	}
}

func TestDefaultBlockedImagePatterns(t *testing.T) {
	patterns := map[string]bool{}
	for _, pattern := range blockedURLPatterns {
		patterns[pattern] = true
	}
	for _, pattern := range []string{"*.jpg*", "*.png*", "*.webp*", "*sns-img*"} {
		if !patterns[pattern] {
			t.Fatalf("缺少图片拦截规则 %q", pattern)
		}
	}
	for _, pattern := range []string{"*sinaimg.cn*", "*doubanio.com*"} {
		if patterns[pattern] {
			t.Fatalf("不能整域拦截 %q:该域同时承载页面 JS/CSS", pattern)
		}
	}
}

// netRecorder 记录 Network.enable 参数、拦截表与 disable 次数。
type netRecorder struct {
	mu       sync.Mutex
	enable   map[string]float64
	blocked  []string
	disables int
}

func installNetworkRecorder(f *fakeCDP, r *netRecorder) {
	f.handle("Network.enable", func(c cdpCommand) {
		var params map[string]float64
		_ = json.Unmarshal(c.Params, &params)
		r.mu.Lock()
		r.enable = params
		r.mu.Unlock()
		f.respond(c.ID, map[string]any{})
	})
	f.handle("Network.setBlockedURLs", func(c cdpCommand) {
		var params struct {
			URLs []string `json:"urls"`
		}
		_ = json.Unmarshal(c.Params, &params)
		r.mu.Lock()
		r.blocked = params.URLs
		r.mu.Unlock()
		f.respond(c.ID, map[string]any{})
	})
	f.handle("Network.disable", func(c cdpCommand) {
		r.mu.Lock()
		r.disables++
		r.mu.Unlock()
		f.respond(c.ID, map[string]any{})
	})
}

type bodyRecorder struct {
	mu  sync.Mutex
	ids []string
}

func (r *bodyRecorder) add(id string) {
	r.mu.Lock()
	r.ids = append(r.ids, id)
	r.mu.Unlock()
}

func (r *bodyRecorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ids...)
}

func installCaptureEval(f *fakeCDP) {
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		var params struct {
			Expression string `json:"expression"`
		}
		_ = json.Unmarshal(c.Params, &params)
		if params.Expression == "performance.timeOrigin" {
			f.respond(c.ID, cdpNumber(1))
			return
		}
		f.respond(c.ID, cdpBool(true))
	})
}

func installCaptureBodies(f *fakeCDP, rec *bodyRecorder) {
	f.handle("Network.getResponseBody", func(c cdpCommand) {
		var params struct {
			RequestID string `json:"requestId"`
		}
		_ = json.Unmarshal(c.Params, &params)
		rec.add(params.RequestID)
		switch params.RequestID {
		case "r1":
			f.respond(c.ID, map[string]any{"body": `{"ok":true}`})
		case "r3":
			f.respond(c.ID, map[string]any{"body": "aGVsbG8=", "base64Encoded": true})
		case "r5":
			f.respond(c.ID, map[string]any{"body": "nomime"})
		default:
			f.respond(c.ID, map[string]any{"body": "unexpected"})
		}
	})
}

func emitCaptureTraffic(f *fakeCDP, base string) {
	f.handle("Page.navigate", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"frameId": "f1", "loaderId": "l1"})
		session := f.currentSession()
		api := base + "/api"
		emitRequest(f, session, "r1", api+"/users?q=x")
		emitResponse(f, session, "r1", "application/json")
		emitFinished(f, session, "r1")
		emitRequest(f, session, "r2", base+"/logo.png")
		emitResponse(f, session, "r2", "image/png")
		emitFinished(f, session, "r2")
		emitRequest(f, session, "r3", api+"/notes")
		emitRequest(f, session, "r3", api+"/final")
		emitResponse(f, session, "r3", "application/json")
		emitFinished(f, session, "r3")
		// r5 没有 mimeType:应保守地尝试取 body。
		emitRequest(f, session, "r5", api+"/nomime")
		emitResponse(f, session, "r5", "")
		emitFinished(f, session, "r5")
		// 其他 session 的事件不属于本 target,必须被过滤。
		emitRequest(f, "other-session", "r6", api+"/other")
		emitResponse(f, "other-session", "r6", "application/json")
		emitFinished(f, "other-session", "r6")
		// r4 永不结束 → idle 永不满足 → 走 timeout 分支。
		emitRequest(f, session, "r4", api+"/hang")
	})
}

func emitRequest(f *fakeCDP, session, id, rawURL string) {
	f.event(session, "Network.requestWillBeSent", map[string]any{
		"requestId": id, "request": map[string]any{"url": rawURL},
	})
}

func emitResponse(f *fakeCDP, session, id, mime string) {
	f.event(session, "Network.responseReceived", map[string]any{
		"requestId": id, "response": map[string]any{"mimeType": mime},
	})
}

func emitFinished(f *fakeCDP, session, id string) {
	f.event(session, "Network.loadingFinished", map[string]any{"requestId": id})
}

// TestCaptureNavigateIdleExit 覆盖「无挂起请求时按 idle 正常退出」分支。
func TestCaptureNavigateIdleExit(t *testing.T) {
	f := newFakeCDP(t)
	installCaptureEval(f)
	installCaptureBodies(f, &bodyRecorder{})
	f.handle("Page.navigate", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"frameId": "f1", "loaderId": "l1"})
		session := f.currentSession()
		emitRequest(f, session, "r1", f.srv.URL+"/api/users")
		emitResponse(f, session, "r1", "application/json")
		emitFinished(f, session, "r1")
	})
	b := newFakeBridge(t, f)
	start := time.Now()
	var got []platform.CapturedResponse
	err := b.Do(context.Background(), f.srv.URL, func(p platform.BrowserPage) error {
		var err error
		got, err = p.CaptureNavigate(context.Background(), f.srv.URL+"/page",
			func(u *url.URL) bool { return strings.HasPrefix(u.Path, "/api/") },
			120*time.Millisecond, 3*time.Second)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Body != `{"ok":true}` {
		t.Fatalf("captured = %+v", got)
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("idle 未生效,耗时 %s", elapsed)
	}
}

// TestCapture 在现有文档上由 trigger 触发请求并收集,不导航。
func TestCapture(t *testing.T) {
	f := newFakeCDP(t)
	installCaptureEval(f)
	installCaptureBodies(f, &bodyRecorder{})
	installNetworkRecorder(f, &netRecorder{})
	b := newFakeBridge(t, f)
	var got []platform.CapturedResponse
	triggered := false
	err := b.Do(context.Background(), f.srv.URL, func(p platform.BrowserPage) error {
		var err error
		got, err = p.Capture(context.Background(), func() error {
			triggered = true
			session := f.currentSession()
			emitRequest(f, session, "r1", f.srv.URL+"/api/users")
			emitResponse(f, session, "r1", "application/json")
			emitFinished(f, session, "r1")
			return nil
		}, func(u *url.URL) bool { return strings.HasPrefix(u.Path, "/api/") },
			120*time.Millisecond, 3*time.Second)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !triggered || len(got) != 1 || got[0].Body != `{"ok":true}` {
		t.Fatalf("triggered/captured = %v/%+v", triggered, got)
	}
}

// TestCaptureNavigateConnClosed 连接在捕获期间断开时必须报 ErrBrowserUnavailable,
// 不能以「成功 + 空结果」返回。
func TestCaptureNavigateConnClosed(t *testing.T) {
	f := newFakeCDP(t)
	installCaptureEval(f)
	f.handle("Page.navigate", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"frameId": "f1", "loaderId": "l1"})
	})
	b := newFakeBridge(t, f)
	err := b.Do(context.Background(), "http://closed.test", func(bp platform.BrowserPage) error {
		p := bp.(*page)
		capCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := p.CaptureNavigate(capCtx, "http://closed.test/page",
				func(*url.URL) bool { return true }, time.Second, 5*time.Second)
			done <- err
		}()
		time.Sleep(150 * time.Millisecond)
		p.conn.close()
		select {
		case err := <-done:
			return err
		case <-time.After(2 * time.Second):
			return errors.New("连接关闭后 CaptureNavigate 未及时返回")
		}
	})
	if !errors.Is(err, platform.ErrBrowserUnavailable) {
		t.Fatalf("err = %v, want ErrBrowserUnavailable", err)
	}
}
