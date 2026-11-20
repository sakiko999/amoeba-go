// Bridge 单测:Evaluate 解码/异常、origin 规范化、同 origin 串行与跨 origin 并发。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestNormalizeOrigin(t *testing.T) {
	cases := []struct {
		in   string
		want string
		bad  bool
	}{
		{"https://www.xiaohongshu.com/", "https://www.xiaohongshu.com", false},
		{"HTTPS://WWW.XiaoHongShu.COM/path?q=1#x", "https://www.xiaohongshu.com", false},
		{"http://127.0.0.1:8123/search", "http://127.0.0.1:8123", false},
		{"http://127.0.0.1:8124", "http://127.0.0.1:8124", false},
		{"ftp://example.com/x", "", true},
		{"not-a-url", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeOrigin(tc.in)
		if tc.bad {
			if err == nil {
				t.Fatalf("normalizeOrigin(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("normalizeOrigin(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestBridgeEvaluateValues(t *testing.T) {
	f, flagsOK := newEvalFake(t)
	b := newFakeBridge(t, f)
	err := b.Do(context.Background(), "http://FAKE.test/ignored?x=1", func(p platform.BrowserPage) error {
		ctx := context.Background()
		if v, err := p.Evaluate(ctx, "1+1"); err != nil || v != float64(2) {
			t.Fatalf("1+1 = %v, %v", v, err)
		}
		if v, err := p.Evaluate(ctx, "text"); err != nil || v != "hi" {
			t.Fatalf("text = %v, %v", v, err)
		}
		obj, err := p.Evaluate(ctx, "obj")
		if err != nil || obj.(map[string]any)["a"] != float64(1) {
			t.Fatalf("obj = %v, %v", obj, err)
		}
		if v, err := p.Evaluate(ctx, "undef"); err != nil || v != nil {
			t.Fatalf("undef = %v, %v", v, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !flagsOK.Load() {
		t.Fatal("Evaluate 必须带 awaitPromise=true 与 returnByValue=true")
	}
}

func TestBridgeEvaluateErrors(t *testing.T) {
	f, _ := newEvalFake(t)
	b := newFakeBridge(t, f)
	err := b.Do(context.Background(), "http://fake.test", func(p platform.BrowserPage) error {
		ctx := context.Background()
		_, err := p.Evaluate(ctx, "boom")
		var evalErr *platform.BrowserEvalError
		if !errors.As(err, &evalErr) || !strings.Contains(evalErr.Description, "Error: boom") {
			t.Fatalf("boom err = %v", err)
		}
		if evalErr.Expression != "boom" {
			t.Fatalf("boom expression = %q", evalErr.Expression)
		}
		if _, err := p.Evaluate(ctx, "nan"); !errors.As(err, &evalErr) {
			t.Fatalf("nan err = %v, want BrowserEvalError", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBridgeResetIdleClosesTargets(t *testing.T) {
	f := newFakeCDP(t)
	var closed atomic.Int32
	f.handle("Target.closeTarget", func(c cdpCommand) {
		closed.Add(1)
		f.respond(c.ID, map[string]any{"success": true})
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()
	for _, origin := range []string{"http://a.test", "http://b.test"} {
		if err := b.Do(ctx, origin, func(platform.BrowserPage) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	b.mu.Lock()
	before := len(b.tabs)
	b.mu.Unlock()
	if before != 2 {
		t.Fatalf("tabs=%d, want 2", before)
	}
	if err := b.ResetIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if got := closed.Load(); got != 2 {
		t.Fatalf("closed=%d, want 2", got)
	}
	b.mu.Lock()
	after := len(b.tabs)
	b.mu.Unlock()
	if after != 0 {
		t.Fatalf("ResetIdle 后缓存 target=%d, want 0", after)
	}
	if err := b.Do(ctx, "http://a.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	rebuilt := len(b.tabs)
	b.mu.Unlock()
	if rebuilt != 1 {
		t.Fatalf("回空页后新任务应重建 target, tabs=%d", rebuilt)
	}
}

func TestBridgeDoSerialPerOrigin(t *testing.T) {
	f := newFakeCDP(t)
	var active, peak atomic.Int32
	installBlockHandler(f, &active, &peak, 120*time.Millisecond)
	b := newFakeBridge(t, f)
	runConcurrentDo(t, b, []string{"http://serial.test", "http://serial.test"})
	if peak.Load() != 1 {
		t.Fatalf("同 origin 并发峰值 = %d, want 1", peak.Load())
	}
}

func TestBridgeDoConcurrentAcrossOrigins(t *testing.T) {
	f := newFakeCDP(t)
	var active, peak atomic.Int32
	installBlockHandler(f, &active, &peak, 150*time.Millisecond)
	b := newFakeBridge(t, f)
	runConcurrentDo(t, b, []string{"http://a.test", "http://b.test"})
	if peak.Load() < 2 {
		t.Fatalf("跨 origin 未并发, 峰值 = %d", peak.Load())
	}
}

// newEvalFake 注册固定的 Runtime.evaluate 应答,并记录参数标志。
func newEvalFake(t *testing.T) (*fakeCDP, *atomic.Bool) {
	t.Helper()
	f := newFakeCDP(t)
	flagsOK := &atomic.Bool{}
	flagsOK.Store(true)
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		var params struct {
			Expression    string `json:"expression"`
			AwaitPromise  bool   `json:"awaitPromise"`
			ReturnByValue bool   `json:"returnByValue"`
		}
		_ = json.Unmarshal(c.Params, &params)
		if !params.AwaitPromise || !params.ReturnByValue {
			flagsOK.Store(false)
		}
		serveEval(f, c.ID, params.Expression)
	})
	return f, flagsOK
}

func serveEval(f *fakeCDP, id int, expr string) {
	switch expr {
	case "1+1":
		f.respond(id, cdpNumber(2))
	case "text":
		f.respond(id, cdpString("hi"))
	case "obj":
		f.respond(id, map[string]any{
			"result": map[string]any{"type": "object", "value": map[string]any{"a": float64(1)}},
		})
	case "undef":
		f.respond(id, map[string]any{"result": map[string]any{"type": "undefined"}})
	case "boom":
		f.respond(id, map[string]any{
			"result":           map[string]any{"type": "object", "subtype": "error", "description": "Error: boom"},
			"exceptionDetails": map[string]any{"text": "Uncaught", "exception": map[string]any{"description": "Error: boom"}},
		})
	case "nan":
		f.respond(id, map[string]any{"result": map[string]any{"type": "number", "description": "NaN"}})
	default:
		f.respond(id, map[string]any{"result": map[string]any{"type": "undefined"}})
	}
}

func installBlockHandler(f *fakeCDP, active, peak *atomic.Int32, delay time.Duration) {
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		var params struct {
			Expression string `json:"expression"`
		}
		_ = json.Unmarshal(c.Params, &params)
		if params.Expression != "block" {
			f.respond(c.ID, cdpNumber(1))
			return
		}
		n := active.Add(1)
		trackPeak(active, peak, n)
		go func() {
			time.Sleep(delay)
			active.Add(-1)
			f.respond(c.ID, cdpNumber(1))
		}()
	})
}

func runConcurrentDo(t *testing.T, b *Bridge, origins []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, len(origins))
	for _, origin := range origins {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- b.Do(ctx, origin, func(p platform.BrowserPage) error {
				_, err := p.Evaluate(ctx, "block")
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func trackPeak(active, peak *atomic.Int32, n int32) {
	for {
		old := peak.Load()
		if n <= old || peak.CompareAndSwap(old, n) {
			return
		}
	}
}
