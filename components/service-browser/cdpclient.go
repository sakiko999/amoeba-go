// CDP 客户端:一条 browser 级 WebSocket 上的命令/事件复用。单读 goroutine
// 按 id 分发回复;写侧一把互斥(gorilla 只允许单个并发 writer);每条命令
// 30s 硬超时并与调用方 ctx 竞速。事件按 sessionId 订阅(CaptureNavigate),
// Target 生命周期事件交给桥接层回调,回调只删缓存、不取 origin 锁。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"mygo-host/platform"
)

const (
	defaultCDPTimeout      = 30 * time.Second
	maxConsecutiveTimeouts = 3
)

// errCDPTargetGone 服务端 -32000 且消息表明 target/session 已消失。
var errCDPTargetGone = errors.New("CDP target 已失效")

type cdpMessage struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	Error     *cdpError       `json:"error"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cdpError) Error() string { return fmt.Sprintf("CDP error %d: %s", e.Code, e.Message) }

// Unwrap 把「target 已消失」的 -32000 映射到 errCDPTargetGone。
func (e *cdpError) Unwrap() error {
	if e.Code != -32000 {
		return nil
	}
	msg := strings.ToLower(e.Message)
	for _, marker := range []string{"no target", "target closed", "session closed"} {
		if strings.Contains(msg, marker) {
			return errCDPTargetGone
		}
	}
	return nil
}

type cdpReply struct {
	result json.RawMessage
	err    error
}

// cdpEvent 无 id 的事件帧(Target.* / Network.*)。
type cdpEvent struct {
	Method    string
	SessionID string
	Params    json.RawMessage
}

type cdpConn struct {
	wsURL string
	gen   uint64
	conn  *websocket.Conn

	cmdTimeout time.Duration // 单测可缩短

	writeMu sync.Mutex

	mu       sync.Mutex
	nextID   int
	pending  map[int]chan cdpReply
	subs     map[int]func(cdpEvent)
	nextSub  int
	closed   bool
	timeouts int // 连续无回复的命令超时数,用于识别假死连接

	onEvent func(cdpEvent)
}

// dialCDP 拨号并启动读循环;onEvent 可为 nil(单测)。
func dialCDP(ctx context.Context, wsURL string, gen uint64, onEvent func(cdpEvent)) (*cdpConn, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	ws, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	c := &cdpConn{
		wsURL:      wsURL,
		gen:        gen,
		conn:       ws,
		cmdTimeout: defaultCDPTimeout,
		pending:    map[int]chan cdpReply{},
		subs:       map[int]func(cdpEvent){},
		onEvent:    onEvent,
	}
	go c.readLoop()
	return c, nil
}

func (c *cdpConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *cdpConn) close() { c.markClosed() }

// call 发一条命令并等回复;30s 硬超时与 ctx 先到者生效。
func (c *cdpConn) call(ctx context.Context, method string, params any, sessionID string, out any) error {
	id, ch, err := c.register()
	if err != nil {
		return err
	}
	defer c.dropPending(id)
	if err := c.writeCommand(id, method, params, sessionID); err != nil {
		return err
	}
	timer := time.NewTimer(c.requestTimeout())
	defer timer.Stop()
	select {
	case reply := <-ch:
		return decodeCDPReply(reply, method, out)
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		c.noteTimeout()
		return fmt.Errorf("%w: CDP 命令超时(%s)", platform.ErrBrowserTimeout, method)
	}
}

// noteTimeout 连续多次无回复说明连接假死(TCP 未断但浏览器不处理命令),
// 主动断开以触发下一次 Do 重连;单次超时不立即断连。
func (c *cdpConn) noteTimeout() {
	c.mu.Lock()
	c.timeouts++
	dead := c.timeouts >= maxConsecutiveTimeouts
	c.mu.Unlock()
	if dead {
		c.markClosed()
	}
}

func (c *cdpConn) register() (int, chan cdpReply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, nil, fmt.Errorf("%w: CDP 连接已关闭", platform.ErrBrowserUnavailable)
	}
	c.nextID++
	id := c.nextID
	ch := make(chan cdpReply, 1)
	c.pending[id] = ch
	return id, ch, nil
}

func (c *cdpConn) dropPending(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *cdpConn) requestTimeout() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cmdTimeout
}

func (c *cdpConn) writeCommand(id int, method string, params any, sessionID string) error {
	req := struct {
		ID        int    `json:"id"`
		Method    string `json:"method"`
		Params    any    `json:"params,omitempty"`
		SessionID string `json:"sessionId,omitempty"`
	}{ID: id, Method: method, Params: params, SessionID: sessionID}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	// 写侧必须有 deadline:对端不读时 WriteJSON 会永久阻塞并一直占着写锁。
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.requestTimeout()))
	if err := c.conn.WriteJSON(req); err != nil {
		c.markClosed()
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return fmt.Errorf("%w: CDP 写入超时: %v", platform.ErrBrowserTimeout, err)
		}
		return fmt.Errorf("%w: CDP 写入失败: %v", platform.ErrBrowserUnavailable, err)
	}
	return nil
}

func decodeCDPReply(reply cdpReply, method string, out any) error {
	if reply.err != nil {
		return reply.err
	}
	if out != nil && len(reply.result) > 0 {
		if err := json.Unmarshal(reply.result, out); err != nil {
			return fmt.Errorf("CDP 结果解码失败(%s): %w", method, err)
		}
	}
	return nil
}

func (c *cdpConn) readLoop() {
	for {
		var msg cdpMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			c.markClosed()
			return
		}
		if msg.ID != 0 {
			c.mu.Lock()
			ch := c.pending[msg.ID]
			delete(c.pending, msg.ID)
			c.timeouts = 0
			c.mu.Unlock()
			if ch == nil {
				continue
			}
			reply := cdpReply{result: msg.Result}
			if msg.Error != nil {
				reply.err = msg.Error
			}
			ch <- reply
			continue
		}
		if msg.Method != "" {
			c.dispatch(cdpEvent{Method: msg.Method, SessionID: msg.SessionID, Params: msg.Params})
		}
	}
}

// dispatch 先回调桥接层(生命周期),再分发给订阅器;两者都不持 c.mu。
func (c *cdpConn) dispatch(ev cdpEvent) {
	if c.onEvent != nil {
		c.onEvent(ev)
	}
	c.mu.Lock()
	subs := make([]func(cdpEvent), 0, len(c.subs))
	for _, fn := range c.subs {
		subs = append(subs, fn)
	}
	c.mu.Unlock()
	for _, fn := range subs {
		fn(ev)
	}
}

// subscribe 注册事件订阅器;返回的函数在 Network.disable 后调用。
func (c *cdpConn) subscribe(fn func(cdpEvent)) func() {
	c.mu.Lock()
	c.nextSub++
	id := c.nextSub
	c.subs[id] = fn
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		delete(c.subs, id)
		c.mu.Unlock()
	}
}

func (c *cdpConn) markClosed() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	pending := c.pending
	c.pending = map[int]chan cdpReply{}
	c.mu.Unlock()
	_ = c.conn.Close()
	for _, ch := range pending {
		select {
		case ch <- cdpReply{err: fmt.Errorf("%w: CDP 连接已关闭", platform.ErrBrowserUnavailable)}:
		default:
		}
	}
}
