// target 缓存的回收侧:Do 结束重置 5 分钟 idle 计时;超过 8 个时异步取
// 受害者 origin 锁做 LRU 回收;Target.* 事件只删缓存,不取 origin 锁。
// ResetIdle 是池级回空页:直接丢弃缓存并关闭全部 target。
// 锁顺序固定为 origin 锁 → Bridge.mu。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type tabEntry struct {
	key       string
	targetID  string
	sessionID string
	conn      *cdpConn // 创建该 target 的连接;重连后旧条目不得复用
	lastUsed  time.Time
	timer     *time.Timer
}

// touch 结束/创建 Do 后重置 idle 计时;回调会取 origin 锁再二次确认。
func (b *Bridge) touch(tab *tabEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tabs[tab.key] != tab {
		return
	}
	tab.lastUsed = time.Now()
	if tab.timer != nil {
		tab.timer.Stop()
	}
	tab.timer = time.AfterFunc(tabIdleClose, func() { b.idleClose(tab.key, tab) })
}

func (b *Bridge) idleClose(key string, tab *tabEntry) {
	l := b.keyLock(key)
	_ = l.lock(context.Background())
	defer l.unlock()
	b.mu.Lock()
	if b.tabs[key] != tab || time.Since(tab.lastUsed) < tabIdleClose {
		b.mu.Unlock()
		return
	}
	delete(b.tabs, key)
	conn := b.conn
	b.mu.Unlock()
	b.closeTarget(conn, tab.targetID)
}

// evictLRU 异步取受害者 origin 锁,避免持有当前 origin 锁去取另一把锁。
func (b *Bridge) evictLRU(except string) {
	b.mu.Lock()
	var victim *tabEntry
	for key, tab := range b.tabs {
		if key == except {
			continue
		}
		if victim == nil || tab.lastUsed.Before(victim.lastUsed) {
			victim = tab
		}
	}
	var pickedAt time.Time
	if victim != nil {
		pickedAt = victim.lastUsed
	}
	b.mu.Unlock()
	if victim == nil {
		return
	}
	go func() {
		l := b.keyLock(victim.key)
		_ = l.lock(context.Background())
		defer l.unlock()
		b.mu.Lock()
		// 等锁期间该 origin 可能刚完成一次 Do:被用过就不淘汰,保住 SPA 状态。
		if b.tabs[victim.key] != victim || !victim.lastUsed.Equal(pickedAt) {
			b.mu.Unlock()
			return
		}
		delete(b.tabs, victim.key)
		conn := b.conn
		b.mu.Unlock()
		if victim.timer != nil {
			victim.timer.Stop()
		}
		b.closeTarget(conn, victim.targetID)
	}()
}

// ResetIdle 关闭全部缓存的 target,让空闲载具回到基础 about:blank 空页
// (进程与 profile 保留)。池在实例闲置时调用;并发 Do 由池的 busy 状态排除,
// 重置期间池会阻止新租约。CDP 失败只影响资源回收,不影响后续任务。
func (b *Bridge) ResetIdle(ctx context.Context) error {
	b.mu.Lock()
	tabs := b.tabs
	b.tabs = map[string]*tabEntry{}
	conn := b.conn
	b.mu.Unlock()
	for _, tab := range tabs {
		if tab.timer != nil {
			tab.timer.Stop()
		}
	}
	if conn == nil || conn.isClosed() || len(tabs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for _, tab := range tabs {
		if conn.isClosed() {
			break
		}
		_ = conn.call(ctx, "Target.closeTarget", map[string]any{"targetId": tab.targetID}, "", nil)
	}
	return nil
}

func (b *Bridge) closeTarget(conn *cdpConn, targetID string) {
	if conn == nil || conn.isClosed() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = conn.call(ctx, "Target.closeTarget", map[string]any{"targetId": targetID}, "", nil)
}

// handleEvent 只删 target 缓存;不取 origin 锁(避免与 Do 反序加锁)。
func (b *Bridge) handleEvent(ev cdpEvent) {
	switch ev.Method {
	case "Target.detachedFromTarget", "Target.targetDestroyed":
	default:
		return
	}
	var params struct {
		TargetID string `json:"targetId"`
	}
	if json.Unmarshal(ev.Params, &params) != nil || params.TargetID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, tab := range b.tabs {
		if tab.targetID != params.TargetID {
			continue
		}
		delete(b.tabs, key)
		if tab.timer != nil {
			tab.timer.Stop()
		}
	}
}

// createTarget 的屏外参数对齐 old 生产驱动,避免窗口在屏幕中央闪一下。
func (c *cdpConn) createTarget(ctx context.Context) (string, error) {
	var out struct {
		TargetID string `json:"targetId"`
	}
	err := c.call(ctx, "Target.createTarget", map[string]any{
		"url":        "about:blank",
		"newWindow":  true,
		"left":       -32000,
		"top":        -32000,
		"width":      1280,
		"height":     800,
		"background": true,
	}, "", &out)
	if err != nil {
		return "", err
	}
	if out.TargetID == "" {
		return "", errors.New("CDP: Target.createTarget 未返回 targetId")
	}
	return out.TargetID, nil
}

func (c *cdpConn) attach(ctx context.Context, targetID string) (string, error) {
	var out struct {
		SessionID string `json:"sessionId"`
	}
	err := c.call(ctx, "Target.attachToTarget",
		map[string]any{"targetId": targetID, "flatten": true}, "", &out)
	if err != nil {
		return "", err
	}
	if out.SessionID == "" {
		return "", errors.New("CDP: Target.attachToTarget 未返回 sessionId")
	}
	return out.SessionID, nil
}
