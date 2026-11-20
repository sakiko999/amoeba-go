// 池调度单测:同平台串行、冷却后复用、常驻实例回空页、容量上限。
// 载具启动用假 newInstance 注入,不拉 WebView2。

package browser

import (
	"context"
	"sync"
	"testing"
	"time"
)

type poolHarness struct {
	p       *Pool
	mu      sync.Mutex
	created map[string]int
	stopped map[string]int
	resets  map[string]int
}

func newPoolHarness(t *testing.T, max int, cooldown time.Duration) *poolHarness {
	t.Helper()
	h := &poolHarness{
		created: map[string]int{}, stopped: map[string]int{}, resets: map[string]int{},
	}
	p := NewPool(t.TempDir(), t.Logf)
	p.max, p.cooldown = max, cooldown
	p.newInstance = func(_ context.Context, profile string) (*instance, error) {
		h.mu.Lock()
		h.created[profile]++
		h.mu.Unlock()
		return &instance{
			profile: profile,
			stop: func() error {
				h.mu.Lock()
				h.stopped[profile]++
				h.mu.Unlock()
				return nil
			},
			reset: func(context.Context) error {
				h.mu.Lock()
				h.resets[profile]++
				h.mu.Unlock()
				return nil
			},
		}, nil
	}
	h.p = p
	return h
}

func (h *poolHarness) counts(profile string) (created, stopped, resets int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.created[profile], h.stopped[profile], h.resets[profile]
}

func TestPoolKeepsWarmInstanceAfterCooldown(t *testing.T) {
	h := newPoolHarness(t, 4, 100*time.Millisecond)
	ctx := context.Background()
	l1, err := h.p.Acquire(ctx, "weibo")
	if err != nil {
		t.Fatal(err)
	}
	l1.Release()
	l2, err := h.p.Acquire(ctx, "weibo")
	if err != nil {
		t.Fatal(err)
	}
	if l2.inst != l1.inst {
		t.Fatal("冷却期内应复用同一实例")
	}
	l2.Release()
	time.Sleep(300 * time.Millisecond)
	if created, stopped, resets := h.counts("weibo"); created != 1 || stopped != 0 || resets != 1 {
		t.Fatalf("created=%d stopped=%d resets=%d, want 1/0/1", created, stopped, resets)
	}
	if st := h.p.Status(); len(st.Instances) != 1 || !st.Instances[0].Warm {
		t.Fatalf("常驻实例状态 = %+v", st)
	}
	l3, err := h.p.Acquire(ctx, "weibo")
	if err != nil {
		t.Fatal(err)
	}
	if l3.inst != l1.inst {
		t.Fatal("常驻实例应直接复用,不应冷启动")
	}
	l3.Release()
}

func TestPoolKeepsOnlyLatestWarm(t *testing.T) {
	h := newPoolHarness(t, 4, 50*time.Millisecond)
	ctx := context.Background()
	la, err := h.p.Acquire(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	la.Release()
	time.Sleep(30 * time.Millisecond)
	lb, err := h.p.Acquire(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	lb.Release()
	time.Sleep(250 * time.Millisecond)
	if created, stopped, _ := h.counts("a"); created != 1 || stopped != 1 {
		t.Fatalf("a created=%d stopped=%d, want 1/1", created, stopped)
	}
	if created, stopped, resets := h.counts("b"); created != 1 || stopped != 0 || resets != 1 {
		t.Fatalf("b created=%d stopped=%d resets=%d, want 1/0/1", created, stopped, resets)
	}
}

func TestPoolDoesNotResetWithinCooldown(t *testing.T) {
	h := newPoolHarness(t, 4, 200*time.Millisecond)
	ctx := context.Background()
	l1, err := h.p.Acquire(ctx, "xhs")
	if err != nil {
		t.Fatal(err)
	}
	l1.Release()
	time.Sleep(50 * time.Millisecond)
	if _, _, resets := h.counts("xhs"); resets != 0 {
		t.Fatalf("冷却期内不应回空页, resets=%d", resets)
	}
	l2, err := h.p.Acquire(ctx, "xhs")
	if err != nil {
		t.Fatal(err)
	}
	if l2.inst != l1.inst {
		t.Fatal("冷却结束后应复用同一实例")
	}
	if _, _, resets := h.counts("xhs"); resets != 0 {
		t.Fatalf("排队中的任务不应触发回空页, resets=%d", resets)
	}
	l2.Release()
}

func TestPoolSameProfileSerializes(t *testing.T) {
	h := newPoolHarness(t, 4, 20*time.Millisecond)
	ctx := context.Background()
	l1, err := h.p.Acquire(ctx, "xhs")
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan *Lease, 1)
	go func() {
		l, _ := h.p.Acquire(ctx, "xhs")
		got <- l
	}()
	select {
	case <-got:
		t.Fatal("同 profile 在占用期间不应并发取得实例")
	case <-time.After(80 * time.Millisecond):
	}
	l1.Release()
	select {
	case l2 := <-got:
		if l2 == nil || l2.inst != l1.inst {
			t.Fatalf("第二个租约 = %v", l2)
		}
		l2.Release()
	case <-time.After(2 * time.Second):
		t.Fatal("释放后未能取得租约")
	}
	if created, _, _ := h.counts("xhs"); created != 1 {
		t.Fatalf("created=%d, want 1", created)
	}
}

func TestPoolCapacityTrimmedByRelease(t *testing.T) {
	h := newPoolHarness(t, 2, time.Minute)
	ctx := context.Background()
	l1, err := h.p.Acquire(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.p.Acquire(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	got := make(chan *Lease, 1)
	go func() {
		l, _ := h.p.Acquire(ctx, "c")
		got <- l
	}()
	select {
	case <-got:
		t.Fatal("容量已满时不应再建实例")
	case <-time.After(80 * time.Millisecond):
	}
	l1.Release() // 有 profile 在等容量:立即销毁让位
	select {
	case l3 := <-got:
		if l3 == nil {
			t.Fatal("让位后应取得租约")
		}
		l3.Release()
	case <-time.After(2 * time.Second):
		t.Fatal("让位后仍排队")
	}
	if created, _, _ := h.counts("a"); created != 1 {
		t.Fatalf("a created=%d, want 1", created)
	}
}

func TestPoolStopAll(t *testing.T) {
	h := newPoolHarness(t, 4, time.Minute)
	ctx := context.Background()
	l, err := h.p.Acquire(ctx, "douyin")
	if err != nil {
		t.Fatal(err)
	}
	_ = l
	h.p.StopAll()
	if created, stopped, _ := h.counts("douyin"); created != 1 || stopped != 1 {
		t.Fatalf("created=%d stopped=%d, want 1/1", created, stopped)
	}
	if st := h.p.Status(); st.Running != 0 {
		t.Fatalf("StopAll 后 status = %+v", st)
	}
}
