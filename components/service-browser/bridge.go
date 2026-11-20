// 桥接执行面:把 platform.Browser 的 per-origin Do 映射到载具上的 page target。
// 一条 browser WebSocket 共享给所有 origin;每 origin 一把锁保证 fn 序列原子,
// target 惰性创建并缓存 5 分钟,超过 8 个按 LRU 异步回收。锁顺序固定为
// origin 锁 → Bridge.mu,事件回调只取 Bridge.mu,绝不取 origin 锁。

package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"mygo-host/platform"
)

const (
	tabIdleClose = 5 * time.Minute
	maxTabs      = 8
)

type Bridge struct {
	svc     *Service
	logf    func(string, ...any)
	connect func(ctx context.Context) (*cdpConn, error) // 单测注入假 CDP;nil 走载具

	connectMu sync.Mutex // 只包连接建立,不覆盖 CDP 调用

	mu    sync.Mutex
	conn  *cdpConn
	tabs  map[string]*tabEntry
	locks map[string]*keyLock
}

// keyLock 是可取消的互斥锁:Do/CloseTab 排队等锁时响应 ctx;
// 后台回收线程用无 ctx 的阻塞写法。
type keyLock struct{ sem chan struct{} }

func newKeyLock() *keyLock { return &keyLock{sem: make(chan struct{}, 1)} }

func (l *keyLock) lock(ctx context.Context) error {
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *keyLock) unlock() { <-l.sem }

var _ platform.Browser = (*Bridge)(nil)

// NewBridge 构造一次,经 ServiceContext.Browser 注入平台。
func NewBridge(svc *Service, logf func(string, ...any)) *Bridge {
	if logf == nil && svc != nil {
		logf = svc.logf
	}
	return &Bridge{
		svc:   svc,
		logf:  logf,
		tabs:  map[string]*tabEntry{},
		locks: map[string]*keyLock{},
	}
}

func (b *Bridge) keyLock(key string) *keyLock {
	b.mu.Lock()
	defer b.mu.Unlock()
	l := b.locks[key]
	if l == nil {
		l = newKeyLock()
		b.locks[key] = l
	}
	return l
}

func (b *Bridge) Do(ctx context.Context, origin string, fn func(platform.BrowserPage) error) error {
	return b.DoAs(ctx, origin, origin, fn)
}

// Hold 是单实例场景的空操作:进程不随任务回收,target 由 CloseTabAs 收尾。
// 池化场景的长期占用在 Router.Hold(见 router.go)。
func (b *Bridge) Hold(context.Context, string, string) error { return nil }

// Cookies 与 CookiesAs 是单实例下 AllCookies 的别名,只为满足 credentials 接口。
func (b *Bridge) Cookies(ctx context.Context, _ string) ([]platform.BrowserCookie, error) {
	return b.AllCookies(ctx)
}

func (b *Bridge) CookiesAs(ctx context.Context, _, _ string) ([]platform.BrowserCookie, error) {
	return b.AllCookies(ctx)
}

// DoAs 在自定义 key 的 target 上执行 fn(origin 只用于导航校验)。登录等长流程用
// 独立的 key(如 "login:xhs"),不与搜索共用同一 origin 的 target 和锁。
func (b *Bridge) DoAs(ctx context.Context, key, origin string, fn func(platform.BrowserPage) error) error {
	norm, err := normalizeOrigin(origin)
	if err != nil {
		return err
	}
	if key == "" {
		return errors.New("browser: DoAs 缺少 key")
	}
	if fn == nil {
		return errors.New("browser: DoAs 缺少回调")
	}
	l := b.keyLock(key)
	if err := l.lock(ctx); err != nil {
		return err
	}
	defer l.unlock()

	tab, conn, err := b.acquire(ctx, key)
	if err != nil {
		return err
	}
	p := &page{conn: conn, key: norm, sessionID: tab.sessionID}
	err = fn(p)
	b.touch(tab)
	return err
}

func (b *Bridge) CloseTab(ctx context.Context, origin string) error {
	key, err := normalizeOrigin(origin)
	if err != nil {
		return err
	}
	return b.CloseTabAs(ctx, key)
}

// CloseTabAs 关闭自定义 key 的 target(DoAs 的配套关闭路径),幂等。
func (b *Bridge) CloseTabAs(ctx context.Context, key string) error {
	l := b.keyLock(key)
	if err := l.lock(ctx); err != nil {
		return err
	}
	defer l.unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	tab := b.tabs[key]
	conn := b.conn
	b.mu.Unlock()
	if tab == nil {
		return nil
	}
	if conn == nil || conn.isClosed() {
		// 连接断了但载具可能仍在:补一次重连,由 installConn 补关旧 target。
		if _, err := b.reconnectForClose(ctx); err != nil {
			return nil
		}
	}
	tab, conn = b.dropTab(key)
	if tab == nil || conn == nil || conn.isClosed() {
		return nil
	}
	_ = conn.call(ctx, "Target.closeTarget", map[string]any{"targetId": tab.targetID}, "", nil)
	return nil
}

// reconnectForClose 只用于 CloseTab 的补关:载具已停止或状态正忙时不重连,
// 避免为了关 target 反而拉起新进程;遗留条目由下一次连接切换清理。
func (b *Bridge) reconnectForClose(ctx context.Context) (*cdpConn, error) {
	if b.svc == nil || !b.svc.runningNow() {
		return nil, fmt.Errorf("%w: 载具未运行", platform.ErrBrowserUnavailable)
	}
	return b.ensureConn(ctx)
}

func (b *Bridge) dropTab(key string) (*tabEntry, *cdpConn) {
	b.mu.Lock()
	tab := b.tabs[key]
	delete(b.tabs, key)
	conn := b.conn
	b.mu.Unlock()
	if tab != nil && tab.timer != nil {
		tab.timer.Stop()
	}
	return tab, conn
}

// acquire 复用前校验 target 是否存在;失效只重建一次。
func (b *Bridge) acquire(ctx context.Context, key string) (*tabEntry, *cdpConn, error) {
	conn, err := b.ensureConn(ctx)
	if err != nil {
		return nil, nil, err
	}
	b.mu.Lock()
	tab := b.tabs[key]
	b.mu.Unlock()
	if tab == nil {
		return b.createTab(ctx, conn, key, false)
	}
	if tab.conn != nil && tab.conn != conn {
		return nil, nil, fmt.Errorf("%w: 连接已切换", platform.ErrBrowserUnavailable)
	}
	err = conn.call(ctx, "Target.getTargetInfo", map[string]any{"targetId": tab.targetID}, "", nil)
	if err == nil {
		return tab, conn, nil
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	if !errors.Is(err, errCDPTargetGone) {
		return nil, nil, err
	}
	b.dropTab(key)
	b.closeTarget(conn, tab.targetID)
	return b.createTab(ctx, conn, key, true)
}

func (b *Bridge) createTab(ctx context.Context, conn *cdpConn, key string, rebuilt bool) (*tabEntry, *cdpConn, error) {
	targetID, err := conn.createTarget(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, b.targetErr(key, rebuilt, "创建 page target 失败", err)
	}
	// 新 target 是独立顶层窗口,触发载具立即去任务栏,不等下一个 2s 周期。
	if b.svc != nil {
		b.svc.Tidy()
	}
	sessionID, err := conn.attach(ctx, targetID)
	if err != nil {
		b.closeTarget(conn, targetID)
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, b.targetErr(key, rebuilt, "attach page target 失败", err)
	}
	if err := armDefaultNetwork(ctx, conn, sessionID); err != nil {
		b.closeTarget(conn, targetID)
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, b.targetErr(key, rebuilt, "配置默认资源拦截失败", err)
	}
	tab := &tabEntry{key: key, targetID: targetID, sessionID: sessionID, conn: conn, lastUsed: time.Now()}
	b.mu.Lock()
	if b.conn != conn {
		cur := b.conn
		b.mu.Unlock()
		if cur == nil || cur.isClosed() {
			cur = conn
		}
		b.closeTarget(cur, targetID)
		return nil, nil, fmt.Errorf("%w: 连接已切换", platform.ErrBrowserUnavailable)
	}
	b.tabs[key] = tab
	count := len(b.tabs)
	b.mu.Unlock()
	b.touch(tab)
	if count > maxTabs {
		b.evictLRU(key)
	}
	return tab, conn, nil
}

func (b *Bridge) targetErr(key string, rebuilt bool, what string, err error) error {
	if rebuilt {
		return fmt.Errorf("%w: page target 已消失(%s): %w", platform.ErrBrowserTargetLost, key, err)
	}
	return fmt.Errorf("%w: %s: %w", platform.ErrBrowserUnavailable, what, err)
}

// normalizeOrigin 丢弃 path/query/fragment,保留非默认端口;http/https 之外拒绝。
func normalizeOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("browser: 非法 origin %q", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("browser: 只支持 http/https origin, got %q", raw)
	}
	return urlOrigin(u), nil
}

func urlOrigin(u *url.URL) string {
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}
