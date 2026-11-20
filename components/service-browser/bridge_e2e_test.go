//go:build windows

// 真实 WebView2 桥接 E2E:Do → Navigate → Evaluate(async IIFE) → WaitUntil
// → Cookies → CaptureNavigate(页面自发 fetch) → CloseTab,以及 Stop 后
// 下一次 Do 的重启自愈(旧连接失效、重新拉起载具并重建 target)。

package browser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestBridgeEndToEnd(t *testing.T) {
	svc := newTestService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	bridge := NewBridge(svc, t.Logf)
	t.Cleanup(func() { cleanupBridgeTest(bridge, svc) })

	server, apiHits := newBridgeServer()
	defer server.Close()
	origin := server.URL

	err := bridge.Do(ctx, origin, func(p platform.BrowserPage) error {
		if err := checkBasePage(ctx, p, origin); err != nil {
			return err
		}
		return checkCapture(ctx, p, origin, apiHits)
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if err := bridge.CloseTab(ctx, origin); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	checkRestart(ctx, t, bridge, svc, origin)
}

func newBridgeServer() (*httptest.Server, *atomic.Int32) {
	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/data" {
			hits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true,"n":7}`)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "bridge_probe", Value: "yes"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<title>bridge-e2e</title><script>
			window.__probe = 42;
			fetch('/api/data').catch(() => {});
		</script>`)
	}))
	return server, hits
}

func checkBasePage(ctx context.Context, p platform.BrowserPage, origin string) error {
	if err := p.Navigate(ctx, origin+"/page"); err != nil {
		return err
	}
	v, err := p.Evaluate(ctx, `(async () => { await new Promise((r) => setTimeout(r, 30)); return 7; })()`)
	if err != nil {
		return err
	}
	if v != float64(7) {
		return fmt.Errorf("async IIFE = %v, want 7", v)
	}
	if err := p.WaitUntil(ctx, "window.__probe === 42", 10*time.Second); err != nil {
		return err
	}
	cookies, err := p.Cookies(ctx, origin+"/")
	if err != nil {
		return err
	}
	if cookies["bridge_probe"] != "yes" {
		return fmt.Errorf("cookie bridge_probe = %q", cookies["bridge_probe"])
	}
	return nil
}

func checkCapture(ctx context.Context, p platform.BrowserPage, origin string, apiHits *atomic.Int32) error {
	captured, err := p.CaptureNavigate(ctx, origin+"/page2",
		func(u *url.URL) bool { return u.Path == "/api/data" },
		500*time.Millisecond, 15*time.Second)
	if err != nil {
		return err
	}
	if len(captured) == 0 {
		return fmt.Errorf("CaptureNavigate 未捕获 /api/data(页面 fetch hits=%d)", apiHits.Load())
	}
	if !strings.Contains(captured[0].Body, `"ok":true`) {
		return fmt.Errorf("captured[0] = %+v", captured[0])
	}
	return nil
}

func checkRestart(ctx context.Context, t *testing.T, bridge *Bridge, svc *Service, origin string) {
	t.Helper()
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		bridge.mu.Lock()
		c := bridge.conn
		bridge.mu.Unlock()
		return c == nil || c.isClosed()
	})
	var sum any
	err := bridge.Do(ctx, origin, func(p platform.BrowserPage) error {
		var err error
		sum, err = p.Evaluate(ctx, "1+1")
		return err
	})
	if err != nil {
		t.Fatalf("重启后 Do: %v", err)
	}
	if sum != float64(2) {
		t.Fatalf("重启后 1+1 = %v", sum)
	}
}

func cleanupBridgeTest(bridge *Bridge, svc *Service) {
	// 屏外 target 若仍打开,WebView2 浏览器进程会在宿主退出后滞留并锁住 profile。
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	bridge.mu.Lock()
	keys := make([]string, 0, len(bridge.tabs))
	for key := range bridge.tabs {
		keys = append(keys, key)
	}
	bridge.mu.Unlock()
	for _, key := range keys {
		_ = bridge.CloseTabAs(closeCtx, key)
	}
	closeCancel()
	// 断 CDP:调试连接若仍在,同样会拖住浏览器进程退出。
	bridge.mu.Lock()
	if bridge.conn != nil {
		bridge.conn.close()
	}
	bridge.mu.Unlock()
	// Reset 会重试删除 profile;WebView2 子进程释放文件可能更慢,
	// 再轮询一段,避免 testing.TempDir 的 RemoveAll 一次性清理失败。
	resetCtx, resetCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer resetCancel()
	_ = svc.Reset(resetCtx)
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		if _, err := os.Stat(svc.profile); errors.Is(err, os.ErrNotExist) {
			break
		}
		_ = os.RemoveAll(svc.profile)
		time.Sleep(200 * time.Millisecond)
	}
}
