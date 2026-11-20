// 载具实例池:一个 profile 同一时刻至多一个载具进程(WebView2 user data
// folder 不能并发),不同 profile 最多 4 个实例。任务结束先冷却 1 秒(给站点
// 一个喘息窗口);冷却后保留最近使用的一个空闲实例,关闭其页面 target 回到
// about:blank 空页释放页面资源,其余实例停进程。常驻实例让下一个任务不必再付
// WebView2 冷启动。平台亲和由 profile 键保证(见 profile.go),不是调度启发式。

package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const (
	maxCarrierInstances  = 4
	warmCarrierInstances = 1
	carrierCooldown      = time.Second
)

// instance 是一个载具进程及其 CDP 连接。
type instance struct {
	profile string
	svc     *Service
	bridge  *Bridge
	stop    func() error                // 测试注入;nil 用 svc.Stop
	reset   func(context.Context) error // 测试注入;nil 用 bridge.ResetIdle
}

func (i *instance) stopCarrier() error {
	if i.stop != nil {
		return i.stop()
	}
	return i.svc.Stop()
}

// resetIdle 关闭全部页面 target,让进程留在基础空页上。
func (i *instance) resetIdle(ctx context.Context) error {
	if i.reset != nil {
		return i.reset(ctx)
	}
	if i.bridge == nil {
		return nil
	}
	return i.bridge.ResetIdle(ctx)
}

type pooled struct {
	inst      *instance
	busy      bool
	starting  bool
	resetting bool // 正在回空页;期间新租约排队
	blanked   bool // 已回到空页
	lastEnd   time.Time
	waiters   int
	stopTimer *time.Timer
}

// Pool 管理按 profile 隔离的载具进程集合。
type Pool struct {
	root     string
	logf     func(string, ...any)
	max      int
	warm     int
	cooldown time.Duration

	newInstance func(ctx context.Context, profile string) (*instance, error)

	mu      sync.Mutex
	entries map[string]*pooled
	waiters int
	wake    chan struct{}
}

// NewPool 建池;载具实例由同一二进制 --browser-carrier 拉起。
func NewPool(root string, logf func(string, ...any)) *Pool {
	p := &Pool{
		root: root, logf: logf,
		max: maxCarrierInstances, warm: warmCarrierInstances,
		cooldown: carrierCooldown,
		entries:  map[string]*pooled{}, wake: make(chan struct{}),
	}
	p.newInstance = p.startInstance
	return p
}

func (p *Pool) startInstance(_ context.Context, profile string) (*instance, error) {
	svc := newService(filepath.Join(p.root, profile), p.logf)
	if _, err := svc.Endpoint(context.Background()); err != nil {
		_ = svc.Stop()
		return nil, err
	}
	return &instance{profile: profile, svc: svc, bridge: NewBridge(svc, p.logf)}, nil
}

// Lease 是一次任务对实例的独占占用;Release 后实例进入冷却/回收/常驻空页。
type Lease struct {
	pool    *Pool
	profile string
	inst    *instance
	once    sync.Once
}

func (l *Lease) Bridge() *Bridge { return l.inst.bridge }
func (l *Lease) Profile() string { return l.profile }
func (l *Lease) Release()        { l.once.Do(l.release) }
func (l *Lease) release() {
	p := l.pool
	p.mu.Lock()
	e := p.entries[l.profile]
	if e == nil || e.inst != l.inst || !e.busy {
		p.mu.Unlock()
		return
	}
	e.busy = false
	e.lastEnd = time.Now()
	// 别的 profile 在等容量:立刻让位,不等冷却。
	if p.waiters > e.waiters && len(p.entries) >= p.max {
		delete(p.entries, l.profile)
		p.wakeAllLocked()
		p.mu.Unlock()
		_ = l.inst.stopCarrier()
		return
	}
	e.stopTimer = time.AfterFunc(p.cooldown, p.reapIdle)
	p.wakeAllLocked()
	p.mu.Unlock()
}

// Acquire 为 profile 取一个空闲实例(必要时启动),按冷却排队。
func (p *Pool) Acquire(ctx context.Context, profile string) (*Lease, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p.mu.Lock()
		e := p.entries[profile]
		if e != nil {
			if e.busy || e.starting || e.resetting {
				err := p.waitLocked(ctx, e)
				p.mu.Unlock()
				if err != nil {
					return nil, err
				}
				continue
			}
			if wait := p.cooldown - time.Since(e.lastEnd); wait > 0 {
				e.waiters++
				p.waiters++
				ch := p.wake
				p.mu.Unlock()
				err := waitCooldown(ctx, ch, wait)
				p.mu.Lock()
				e.waiters--
				p.waiters--
				p.mu.Unlock()
				if err != nil {
					return nil, err
				}
				continue
			}
			e.busy = true
			e.blanked = false
			if e.stopTimer != nil {
				e.stopTimer.Stop()
				e.stopTimer = nil
			}
			inst := e.inst
			p.mu.Unlock()
			return &Lease{pool: p, profile: profile, inst: inst}, nil
		}
		if len(p.entries) >= p.max {
			ch := p.wake
			p.waiters++
			p.mu.Unlock()
			err := waitCh(ctx, ch)
			p.mu.Lock()
			p.waiters--
			p.mu.Unlock()
			if err != nil {
				return nil, err
			}
			continue
		}
		e = &pooled{starting: true}
		p.entries[profile] = e
		p.mu.Unlock()

		inst, err := p.newInstance(ctx, profile)
		if err == nil {
			err = ctx.Err()
		}
		p.mu.Lock()
		if err != nil {
			delete(p.entries, profile)
			p.wakeAllLocked()
			p.mu.Unlock()
			if inst != nil {
				_ = inst.stopCarrier()
			}
			return nil, fmt.Errorf("browser: 启动 %s 载具失败: %w", profile, err)
		}
		e.inst = inst
		e.starting = false
		e.busy = true
		p.mu.Unlock()
		return &Lease{pool: p, profile: profile, inst: inst}, nil
	}
}

// waitLocked 让当前 Acquire 在同 profile 忙时排队;返回后调用方必须重试。
func (p *Pool) waitLocked(ctx context.Context, e *pooled) error {
	e.waiters++
	p.waiters++
	ch := p.wake
	p.mu.Unlock()
	err := waitCh(ctx, ch)
	p.mu.Lock()
	e.waiters--
	p.waiters--
	return err
}

// waitCh 等唤醒或 ctx 结束。
func waitCh(ctx context.Context, ch <-chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// waitCooldown 等冷却结束(或被其他状态变化唤醒后重试)。
func waitCooldown(ctx context.Context, ch <-chan struct{}, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ch:
		return nil
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// reapIdle 整理冷却结束的空闲实例:保留最近使用的 warm 个常驻,并把它们
// 关闭页面 target 回到空页;其余停进程。多次触发是幂等的,状态判断都在锁内,
// 停进程/回空页在锁外进行。
func (p *Pool) reapIdle() {
	type idleEntry struct {
		profile string
		e       *pooled
	}
	p.mu.Lock()
	now := time.Now()
	var idle []idleEntry
	for profile, e := range p.entries {
		if e.busy || e.starting || e.waiters > 0 || now.Sub(e.lastEnd) < p.cooldown {
			continue
		}
		idle = append(idle, idleEntry{profile, e})
	}
	if len(idle) == 0 {
		p.mu.Unlock()
		return
	}
	slices.SortFunc(idle, func(a, b idleEntry) int {
		return b.e.lastEnd.Compare(a.e.lastEnd)
	})
	keep := min(p.warm, len(idle))
	var victims []*instance
	for _, it := range idle[keep:] {
		if it.e.stopTimer != nil {
			it.e.stopTimer.Stop()
			it.e.stopTimer = nil
		}
		delete(p.entries, it.profile)
		if it.e.inst != nil {
			victims = append(victims, it.e.inst)
		}
	}
	var blanks []idleEntry
	for _, it := range idle[:keep] {
		if it.e.blanked || it.e.resetting {
			continue
		}
		it.e.resetting = true
		blanks = append(blanks, it)
	}
	p.wakeAllLocked()
	p.mu.Unlock()

	for _, inst := range victims {
		_ = inst.stopCarrier()
	}
	for _, it := range blanks {
		_ = it.e.inst.resetIdle(context.Background())
		p.mu.Lock()
		if p.entries[it.profile] == it.e {
			it.e.resetting = false
			it.e.blanked = true
		}
		p.wakeAllLocked()
		p.mu.Unlock()
	}
}

func (p *Pool) wakeAllLocked() {
	close(p.wake)
	p.wake = make(chan struct{})
}

// StopAll 停掉全部实例(宿主退出/重置)。
func (p *Pool) StopAll() {
	p.mu.Lock()
	entries := p.entries
	p.entries = map[string]*pooled{}
	for _, e := range entries {
		if e.stopTimer != nil {
			e.stopTimer.Stop()
		}
	}
	p.wakeAllLocked()
	p.mu.Unlock()
	for _, e := range entries {
		if e.inst != nil {
			_ = e.inst.stopCarrier()
		}
	}
}

// ResetAll 停掉全部实例并删除 profile 根目录(登录态/缓存全部清除)。
func (p *Pool) ResetAll(ctx context.Context) error {
	p.StopAll()
	var err error
	for range 20 {
		if err = os.RemoveAll(p.root); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("清理载具 profile 失败: %w", err)
}

// InstanceStatus 是一个运行中实例的诊断信息。
type InstanceStatus struct {
	Profile  string `json:"profile"`
	PID      int    `json:"pid,omitempty"`
	Port     int    `json:"port,omitempty"`
	Busy     bool   `json:"busy"`
	Starting bool   `json:"starting,omitempty"`
	Warm     bool   `json:"warm,omitempty"` // 空闲且已回到 about:blank 空页
}

// PoolStatus 是池的只读状态。
type PoolStatus struct {
	Running   int              `json:"running"`
	Instances []InstanceStatus `json:"instances"`
}

// Status 只读当前实例,不启动任何进程。
func (p *Pool) Status() PoolStatus {
	type snap struct {
		item InstanceStatus
		svc  *Service
	}
	p.mu.Lock()
	snaps := make([]snap, 0, len(p.entries))
	for profile, e := range p.entries {
		item := InstanceStatus{
			Profile: profile, Busy: e.busy, Starting: e.starting,
			Warm: e.blanked && !e.busy && !e.starting && !e.resetting,
		}
		var svc *Service
		if e.inst != nil {
			svc = e.inst.svc
		}
		snaps = append(snaps, snap{item, svc})
	}
	p.mu.Unlock()
	out := PoolStatus{Instances: make([]InstanceStatus, 0, len(snaps))}
	for _, s := range snaps {
		if s.svc != nil {
			st := s.svc.Status()
			s.item.PID, s.item.Port = st.PID, st.Port
		}
		out.Instances = append(out.Instances, s.item)
	}
	out.Running = len(out.Instances)
	return out
}
