// Bridge 自愈与排队语义回归:重连补关旧 target、getTargetInfo 只对 target
// 失效重建、非 target 错误不误删缓存、同 origin 等锁响应 ctx、命令 sessionId
// 必须在消息顶层。

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

func TestBridgeReconnectClosesStaleTargets(t *testing.T) {
	f := newFakeCDP(t)
	var mu sync.Mutex
	var closed []string
	f.handle("Target.closeTarget", func(c cdpCommand) {
		var params struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(c.Params, &params)
		mu.Lock()
		closed = append(closed, params.TargetID)
		mu.Unlock()
		f.respond(c.ID, map[string]any{"success": true})
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()
	if err := b.Do(ctx, "http://a.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	old := b.conn
	b.mu.Unlock()
	old.close()
	if err := b.Do(ctx, "http://b.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(closed) != 1 || closed[0] != "t1" {
		t.Fatalf("补关的 target = %v, want [t1](同一进程重连不能漏)", closed)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.tabs) != 1 || b.tabs["http://b.test"] == nil {
		t.Fatalf("重连后缓存 = %v, want 仅 b.test", b.tabs)
	}
}

func TestBridgeTargetSelfHeal(t *testing.T) {
	f := newFakeCDP(t)
	var mu sync.Mutex
	creates, getInfos := 0, 0
	f.handle("Target.createTarget", func(c cdpCommand) {
		mu.Lock()
		creates++
		mu.Unlock()
		f.respond(c.ID, map[string]any{"targetId": f.nextTarget()})
	})
	f.handle("Target.getTargetInfo", func(c cdpCommand) {
		mu.Lock()
		getInfos++
		n := getInfos
		mu.Unlock()
		if n == 1 {
			f.fail(c.ID, -32000, "No target with given id found")
			return
		}
		f.respond(c.ID, map[string]any{"targetInfo": map[string]any{"targetId": "t2"}})
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()
	if err := b.Do(ctx, "http://heal.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := b.Do(ctx, "http://heal.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatalf("target 消失后自愈失败: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if creates != 2 || getInfos != 1 {
		t.Fatalf("creates=%d getTargetInfo=%d, want 2/1", creates, getInfos)
	}
}

func TestBridgeTargetInfoErrorNoRebuild(t *testing.T) {
	f := newFakeCDP(t)
	var creates atomic.Int32
	f.handle("Target.createTarget", func(c cdpCommand) {
		creates.Add(1)
		f.respond(c.ID, map[string]any{"targetId": f.nextTarget()})
	})
	f.handle("Target.getTargetInfo", func(c cdpCommand) {
		f.fail(c.ID, -32602, "Invalid parameters")
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()
	if err := b.Do(ctx, "http://bad.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	err := b.Do(ctx, "http://bad.test", func(platform.BrowserPage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "-32602") {
		t.Fatalf("err = %v, want 原始 CDP 错误", err)
	}
	f.handle("Target.getTargetInfo", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"targetInfo": map[string]any{"targetId": "t1"}})
	})
	if err := b.Do(ctx, "http://bad.test", func(platform.BrowserPage) error { return nil }); err != nil {
		t.Fatalf("缓存应保留并可复用: %v", err)
	}
	if creates.Load() != 1 {
		t.Fatalf("非 target 失效错误触发了重建, creates=%d", creates.Load())
	}
}

func TestBridgeDoWaitHonorsContext(t *testing.T) {
	f := newFakeCDP(t)
	var active atomic.Int32
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		active.Add(1)
		time.Sleep(600 * time.Millisecond)
		active.Add(-1)
		f.respond(c.ID, cdpNumber(1))
	})
	b := newFakeBridge(t, f)
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- b.Do(context.Background(), "http://locked.test", func(p platform.BrowserPage) error {
			_, err := p.Evaluate(context.Background(), "x")
			return err
		})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for active.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := b.Do(ctx, "http://locked.test", func(platform.BrowserPage) error { return nil })
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("排队等锁未响应 ctx,耗时 %s", elapsed)
	}
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

// TestBridgeNavigateGuardAndCloseTab 覆盖:跨 origin 导航拒绝、timeOrigin +
// readyState 守卫组合、CloseTab 幂等。
func TestBridgeNavigateGuardAndCloseTab(t *testing.T) {
	f := newFakeCDP(t)
	var mu sync.Mutex
	var conds []string
	pageNavigates := 0
	f.handle("Page.navigate", func(c cdpCommand) {
		mu.Lock()
		pageNavigates++
		mu.Unlock()
		f.respond(c.ID, map[string]any{"frameId": "f1", "loaderId": "l1"})
	})
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		var params struct {
			Expression string `json:"expression"`
		}
		_ = json.Unmarshal(c.Params, &params)
		if params.Expression == "performance.timeOrigin" {
			f.respond(c.ID, cdpNumber(1))
			return
		}
		if strings.Contains(params.Expression, "performance.timeOrigin !==") {
			mu.Lock()
			conds = append(conds, params.Expression)
			mu.Unlock()
		}
		f.respond(c.ID, cdpBool(true))
	})
	b := newFakeBridge(t, f)
	ctx := context.Background()
	err := b.Do(ctx, "http://guard.test", func(p platform.BrowserPage) error {
		if err := p.Navigate(ctx, "http://other.test/x"); err == nil {
			t.Fatal("跨 origin 导航必须被拒绝")
		}
		return p.Navigate(ctx, "http://guard.test/page")
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if pageNavigates != 1 {
		t.Fatalf("Page.navigate 次数 = %d, want 1", pageNavigates)
	}
	if len(conds) == 0 || !strings.Contains(conds[0], "document.readyState === 'complete'") {
		t.Fatalf("就绪条件 = %v(应含 timeOrigin 守卫与 readyState)", conds)
	}
	mu.Unlock()
	if err := b.CloseTab(ctx, "http://guard.test"); err != nil {
		t.Fatal(err)
	}
	if err := b.CloseTab(ctx, "http://guard.test"); err != nil {
		t.Fatalf("CloseTab 应幂等: %v", err)
	}
}

func TestBridgeCommandSessionID(t *testing.T) {
	f := newFakeCDP(t)
	var ok atomic.Bool
	f.handle("Runtime.evaluate", func(c cdpCommand) {
		if c.SessionID == "s-t1" {
			ok.Store(true)
		}
		f.respond(c.ID, cdpNumber(1))
	})
	b := newFakeBridge(t, f)
	err := b.Do(context.Background(), "http://sid.test", func(p platform.BrowserPage) error {
		_, err := p.Evaluate(context.Background(), "1+1")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok.Load() {
		t.Fatal("命令的 sessionId 不在消息顶层(应为 s-t1)")
	}
}
