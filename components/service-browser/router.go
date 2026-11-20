// Router 把平台层的 Browser 调用映射到实例池:每次 Do/DoAs/读 cookie 都是一次
// 任务(独占一个实例,结束即释放);登录用 Hold 长期占住同一平台实例,直到
// CloseTabAs。平台亲和由 ProfileKey 决定,不再跨 origin 共享一个常驻载具。

package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"mygo-host/platform"
)

// loginPinTTL 是登录会话占住平台实例的上限;页面关闭/崩溃没回调 CloseTabAs 时兜底释放。
const loginPinTTL = 10 * time.Minute

var _ platform.Browser = (*Router)(nil)

type pin struct {
	lease *Lease
	timer *time.Timer
}

type Router struct {
	pool *Pool
	logf func(string, ...any)

	mu     sync.Mutex
	pinned map[string]*pin
}

func NewRouter(pool *Pool, logf func(string, ...any)) *Router {
	if logf == nil {
		logf = pool.logf
	}
	return &Router{pool: pool, logf: logf, pinned: map[string]*pin{}}
}

// Do 在平台实例上执行一次独占任务。
func (r *Router) Do(ctx context.Context, origin string, fn func(platform.BrowserPage) error) error {
	key, err := ProfileKey(origin)
	if err != nil {
		return err
	}
	lease, err := r.acquire(ctx, key)
	if err != nil {
		return err
	}
	defer lease.Release()
	return lease.Bridge().Do(ctx, origin, fn)
}

// CloseTab 是空操作:实例空闲回收时会关闭全部 target,页面状态不跨任务保留。
func (r *Router) CloseTab(context.Context, string) error { return nil }

// DoAs 在 pin 住的登录实例上执行;未 pin 时作为一次性任务执行。
func (r *Router) DoAs(ctx context.Context, key, origin string, fn func(platform.BrowserPage) error) error {
	if lease := r.pinnedLease(key); lease != nil {
		return lease.Bridge().DoAs(ctx, key, origin, fn)
	}
	profile, err := ProfileKey(origin)
	if err != nil {
		return err
	}
	lease, err := r.acquire(ctx, profile)
	if err != nil {
		return err
	}
	defer lease.Release()
	return lease.Bridge().DoAs(ctx, key, origin, fn)
}

// Hold 为登录会话独占平台实例,直到 CloseTabAs;登录 pin 超过 loginPinTTL 自动释放。
func (r *Router) Hold(ctx context.Context, key, origin string) error {
	profile, err := ProfileKey(origin)
	if err != nil {
		return err
	}
	r.mu.Lock()
	if r.pinned[key] != nil {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	lease, err := r.acquire(ctx, profile)
	if err != nil {
		return err
	}
	r.mu.Lock()
	if old := r.pinned[key]; old != nil {
		r.mu.Unlock()
		lease.Release()
		return nil
	}
	p := &pin{lease: lease}
	p.timer = time.AfterFunc(loginPinTTL, func() {
		if err := r.CloseTabAs(context.Background(), key); err != nil && r.logf != nil {
			r.logf("browser: 登录会话 %s 超时释放失败: %v", key, err)
		}
	})
	r.pinned[key] = p
	r.mu.Unlock()
	return nil
}

// CloseTabAs 关闭登录 target 并释放实例;未 pin 时为空操作。
func (r *Router) CloseTabAs(ctx context.Context, key string) error {
	r.mu.Lock()
	p := r.pinned[key]
	delete(r.pinned, key)
	r.mu.Unlock()
	if p == nil {
		return nil
	}
	if p.timer != nil {
		p.timer.Stop()
	}
	err := p.lease.Bridge().CloseTabAs(ctx, key)
	p.lease.Release()
	return err
}

// Cookies 在平台实例上读一次 cookie(短任务)。
func (r *Router) Cookies(ctx context.Context, origin string) ([]platform.BrowserCookie, error) {
	profile, err := ProfileKey(origin)
	if err != nil {
		return nil, err
	}
	lease, err := r.acquire(ctx, profile)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	return lease.Bridge().AllCookies(ctx)
}

// CookiesAs 优先读 pin 住的登录实例,否则退化为一次短任务。
func (r *Router) CookiesAs(ctx context.Context, key, origin string) ([]platform.BrowserCookie, error) {
	if lease := r.pinnedLease(key); lease != nil {
		return lease.Bridge().AllCookies(ctx)
	}
	return r.Cookies(ctx, origin)
}

// ClearCookies 按 hosts 推导 profile,逐个清除(退出登录)。
func (r *Router) ClearCookies(ctx context.Context, hosts []string) (int, error) {
	total := 0
	for _, profile := range profilesForHosts(hosts) {
		lease, err := r.acquire(ctx, profile)
		if err != nil {
			return total, err
		}
		n, err := lease.Bridge().ClearCookies(ctx, hosts)
		lease.Release()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// SetCookies 按 cookie 域名分派到对应平台 profile(HTTP 登录回灌)。
func (r *Router) SetCookies(ctx context.Context, cookies []platform.BrowserCookie) error {
	groups := map[string][]platform.BrowserCookie{}
	for _, c := range cookies {
		host := strings.TrimPrefix(strings.TrimSpace(c.Domain), ".")
		if host == "" {
			continue
		}
		key, err := ProfileKey("https://" + host)
		if err != nil {
			continue
		}
		groups[key] = append(groups[key], c)
	}
	for profile, group := range groups {
		lease, err := r.acquire(ctx, profile)
		if err != nil {
			return err
		}
		err = lease.Bridge().SetCookies(ctx, group)
		lease.Release()
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) pinnedLease(key string) *Lease {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.pinned[key]; p != nil {
		return p.lease
	}
	return nil
}

// acquire 把池错误归一成平台错误语义。
func (r *Router) acquire(ctx context.Context, profile string) (*Lease, error) {
	lease, err := r.pool.Acquire(ctx, profile)
	if err == nil {
		return lease, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, fmt.Errorf("%w: %v", platform.ErrBrowserUnavailable, err)
}

func profilesForHosts(hosts []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimPrefix(strings.TrimSpace(h), ".")
		if h == "" {
			continue
		}
		key, err := ProfileKey("https://" + h)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
