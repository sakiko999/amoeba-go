// 连接生命周期:健康复用、断开后重新取端点。载具进程未变(gen 相同)时旧
// target 仍存活,必须在新连接上补关;gen 变化说明进程重启,target 随进程消失。
// 存活探测与调用方 ctx 解耦,避免过期的 ctx 误杀其他 origin 正在用的载具。

package browser

import (
	"context"
	"fmt"
	"time"

	"mygo-host/platform"
)

// ensureConn 连接健康就不探测、不拉起载具;新连接接管旧 target 缓存。
func (b *Bridge) ensureConn(ctx context.Context) (*cdpConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.connectMu.Lock()
	defer b.connectMu.Unlock()

	b.mu.Lock()
	c := b.conn
	b.mu.Unlock()
	if c != nil && !c.isClosed() {
		return c, nil
	}

	dial := b.connect
	if dial == nil {
		dial = b.dialService
	}
	nc, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	b.installConn(c, nc)
	return nc, nil
}

func (b *Bridge) dialService(ctx context.Context) (*cdpConn, error) {
	if b.svc == nil {
		return nil, fmt.Errorf("%w: 未配置载具服务", platform.ErrBrowserUnavailable)
	}
	snap, err := b.svc.snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", platform.ErrBrowserUnavailable, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := dialCDP(ctx, snap.WS, snap.gen, b.handleEvent)
	if err != nil {
		return nil, fmt.Errorf("%w: 连接载具 CDP 失败: %v", platform.ErrBrowserUnavailable, err)
	}
	if b.logf != nil {
		b.logf("browser bridge: connected gen=%d endpoint=%s", snap.gen, snap.WS)
	}
	return c, nil
}

// installConn 切换连接并清空旧缓存。同一载具进程的旧 target 仍然活着,
// 在新连接上同步补关(共享短超时),否则 renderer 会泄漏到进程退出。
func (b *Bridge) installConn(old, nc *cdpConn) {
	b.mu.Lock()
	b.conn = nc
	tabs := b.tabs
	b.tabs = map[string]*tabEntry{}
	b.mu.Unlock()
	for _, tab := range tabs {
		if tab.timer != nil {
			tab.timer.Stop()
		}
	}
	if old == nil || old.gen != nc.gen || len(tabs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, tab := range tabs {
		if nc.isClosed() {
			return
		}
		_ = nc.call(ctx, "Target.closeTarget", map[string]any{"targetId": tab.targetID}, "", nil)
	}
}
