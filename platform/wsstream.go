// WS 弹幕流原语(对齐 platform-kit runtime/danmaku/stream.ts):订阅即建连、
// 退订即断开、指数退避重连(±15% full jitter)、心跳、认证帧 OnOpen 发送、
// refresh 钩子(bili token 有寿命);DeferredStream 组合异步 setup(退订发生在
// setup 完成前则不再建流,连接零泄漏)。平台差异全部经选项注入。
// 运行时差异:WSConn 不暴露 close code/reason,OnClose 只拿到 0 + 错误文案。

package platform

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// WSStreamOptions WS 流选项(对齐 TS WsStreamOptions)。
type WSStreamOptions struct {
	URL            func() string
	Refresh        func() (bool, error)
	OnOpen         func(*WSConn)
	OnMessage      func([]byte, *WSConn) ([]DanmakuItem, error)
	Heartbeat      func() []byte
	HeartbeatMs    int
	MaxReconnectMs int
	MaxAttempts    int
	OnClose        func(code int, reason string)
}

type wsStream struct {
	ctx     context.Context
	opts    WSStreamOptions
	logf    func(string, ...any)
	onItems func([]DanmakuItem)

	mu       sync.Mutex
	stopped  bool
	closed   bool // 用户主动退订
	conn     *WSConn
	hbStop   chan struct{}
	reconn   *time.Timer
	attempts int
}

// CreateWSStream 返回订阅函数(onItems 收批次;返回值是取消函数)。
func CreateWSStream(ctx context.Context, opts WSStreamOptions, logf func(string, ...any)) DanmakuStream {
	return func(onItems func([]DanmakuItem)) func() {
		s := &wsStream{ctx: ctx, opts: opts, logf: logf, onItems: onItems}
		s.connect()
		return s.stop
	}
}

func (s *wsStream) stop() {
	s.mu.Lock()
	s.stopped = true
	s.closed = true
	conn := s.conn
	hbStop := s.hbStop
	reconn := s.reconn
	s.mu.Unlock()
	closeHeartbeat(hbStop)
	if reconn != nil {
		reconn.Stop()
	}
	safeClose(conn)
}

// send 仅在连接就绪时发送(心跳/回执共用)。
func (s *wsStream) send(data []byte) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Write(data)
	}
}

func safeClose(conn *WSConn) {
	if conn != nil {
		_ = conn.Close()
	}
}

func closeHeartbeat(stop chan struct{}) {
	if stop == nil {
		return
	}
	defer func() { _ = recover() }()
	close(stop)
}

// connect 建连(异步;失败进入重连)。
func (s *wsStream) connect() {
	if s.isStopped() {
		return
	}
	url := s.opts.URL()
	go func() {
		conn, err := Dial(s.ctx, url, nil)
		if err != nil {
			if !s.isStopped() {
				s.logf("ws 握手失败: %v", err)
				s.beforeReconnect()
			}
			return
		}
		s.handleOpen(conn)
	}()
}

func (s *wsStream) handleOpen(conn *WSConn) {
	if s.isStopped() {
		safeClose(conn)
		return
	}
	s.mu.Lock()
	s.conn = conn
	s.attempts = 0
	s.mu.Unlock()
	if s.opts.OnOpen != nil {
		s.opts.OnOpen(conn)
	}
	if s.opts.Heartbeat != nil {
		s.startHeartbeat()
	}
	go s.readLoop(conn)
}

func (s *wsStream) readLoop(conn *WSConn) {
	for {
		_, data, err := conn.Read()
		if err != nil {
			s.handleClose(conn, err)
			return
		}
		s.deliver(conn, data)
	}
}

// deliver 帧解码 → 过滤已退订/空 → 上报;解码错误视为连接不可用(close 触发重连)。
func (s *wsStream) deliver(conn *WSConn, data []byte) {
	items, err := s.opts.OnMessage(data, conn)
	if err != nil {
		if !s.isStopped() && !s.isUserClosed() {
			s.logf("ws frame error: %v", err)
			safeClose(conn)
		}
		return
	}
	if len(items) > 0 && !s.isStopped() {
		s.onItems(items)
	}
}

func (s *wsStream) handleClose(conn *WSConn, err error) {
	s.mu.Lock()
	hbStop := s.hbStop
	s.hbStop = nil
	if s.conn == conn {
		s.conn = nil
	}
	userClosed := s.closed
	stopped := s.stopped
	s.mu.Unlock()
	closeHeartbeat(hbStop)
	safeClose(conn)
	if !userClosed {
		s.logf("ws closed: %v", err)
	}
	if s.opts.OnClose != nil {
		s.opts.OnClose(0, err.Error())
	}
	if !stopped {
		s.beforeReconnect()
	}
}

// beforeReconnect 先刷新凭证(token 有寿命),再排重连;refresh 失败/超限即放弃。
func (s *wsStream) beforeReconnect() {
	if s.isStopped() {
		return
	}
	if s.opts.Refresh != nil {
		ok, err := s.opts.Refresh()
		if err != nil {
			s.logf("刷新凭证失败: %v", err)
			return
		}
		if s.isStopped() || !ok {
			return
		}
	}
	s.scheduleReconnect()
}

func (s *wsStream) scheduleReconnect() {
	if s.isStopped() {
		return
	}
	maxAttempts := s.opts.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 10
	}
	s.mu.Lock()
	attempts := s.attempts
	s.mu.Unlock()
	if attempts >= maxAttempts {
		s.logf("ws 重连放弃(已尝试 %d 次)", attempts)
		return
	}
	maxDelay := s.opts.MaxReconnectMs
	if maxDelay == 0 {
		maxDelay = 30000
	}
	base := math.Min(1000*math.Pow(2, float64(attempts)), float64(maxDelay))
	delay := time.Duration(math.Round(base * (0.85 + rand.Float64()*0.3)))
	s.mu.Lock()
	s.attempts++
	s.reconn = time.AfterFunc(delay*time.Millisecond, s.connect)
	s.mu.Unlock()
	s.logf("ws 重连 #%d(%dms)", attempts+1, delay)
}

func (s *wsStream) startHeartbeat() {
	stop := make(chan struct{})
	s.mu.Lock()
	s.hbStop = stop
	s.mu.Unlock()
	interval := s.opts.HeartbeatMs
	if interval <= 0 {
		interval = 30000
	}
	s.send(s.opts.Heartbeat())
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if s.isStopped() {
					return
				}
				s.send(s.opts.Heartbeat())
			}
		}
	}()
}

func (s *wsStream) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

func (s *wsStream) isUserClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// DeferredStream 异步 setup + build 组合成 DanmakuStream:退订发生在 setup 完成
// 前则不再建流(连接零泄漏);setup 失败经 onError 上报(不传则静默)。
func DeferredStream[T any](
	setup func() (T, error),
	build func(T, func([]DanmakuItem)) func(),
	onError func(error),
) DanmakuStream {
	return func(onItems func([]DanmakuItem)) func() {
		var (
			mu      sync.Mutex
			stopped bool
			unsub   func()
		)
		go func() {
			value, err := setup()
			if err != nil {
				if !deferredStopped(&mu, &stopped) && onError != nil {
					onError(err)
				}
				return
			}
			if deferredStopped(&mu, &stopped) {
				return
			}
			u := build(value, onItems)
			mu.Lock()
			unsub = u
			s := stopped
			mu.Unlock()
			if s && u != nil {
				u()
			}
		}()
		return func() {
			mu.Lock()
			stopped = true
			u := unsub
			mu.Unlock()
			if u != nil {
				u()
			}
		}
	}
}

func deferredStopped(mu *sync.Mutex, stopped *bool) bool {
	mu.Lock()
	defer mu.Unlock()
	return *stopped
}
