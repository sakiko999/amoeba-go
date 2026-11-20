// 假 CDP browser 端点:httptest + gorilla Upgrader。命令按 method 路由到
// 测试提供的 handler,handler 用 respond/fail/event 主动回帧;未注册的方法
// 不回复(用于超时用例)。单测不依赖 WebView2,任何平台都能跑。

package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

type cdpCommand struct {
	ID        int
	SessionID string
	Params    json.RawMessage
}

type fakeCDP struct {
	srv      *httptest.Server
	mu       sync.Mutex
	writeMu  sync.Mutex
	conn     *websocket.Conn
	handlers map[string]func(cdpCommand)
	targets  int
	session  string
}

func newFakeCDP(t *testing.T) *fakeCDP {
	t.Helper()
	f := &fakeCDP{handlers: map[string]func(cdpCommand){}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	f.defaults()
	return f
}

func (f *fakeCDP) serve(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()
	go f.readLoop(conn)
}

func (f *fakeCDP) readLoop(conn *websocket.Conn) {
	for {
		var msg struct {
			ID        int             `json:"id"`
			Method    string          `json:"method"`
			SessionID string          `json:"sessionId"`
			Params    json.RawMessage `json:"params"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		f.mu.Lock()
		handler := f.handlers[msg.Method]
		f.mu.Unlock()
		if handler != nil {
			handler(cdpCommand{ID: msg.ID, SessionID: msg.SessionID, Params: msg.Params})
		}
	}
}

func (f *fakeCDP) handle(method string, fn func(cdpCommand)) {
	f.mu.Lock()
	f.handlers[method] = fn
	f.mu.Unlock()
}

func (f *fakeCDP) send(v any) {
	f.mu.Lock()
	conn := f.conn
	f.mu.Unlock()
	if conn == nil {
		return
	}
	f.writeMu.Lock()
	defer f.writeMu.Unlock()
	_ = conn.WriteJSON(v)
}

func (f *fakeCDP) respond(id int, result any) {
	f.send(map[string]any{"id": id, "result": result})
}

func (f *fakeCDP) fail(id, code int, message string) {
	f.send(map[string]any{"id": id, "error": map[string]any{"code": code, "message": message}})
}

func (f *fakeCDP) event(sessionID, method string, params any) {
	f.send(map[string]any{"method": method, "params": params, "sessionId": sessionID})
}

func (f *fakeCDP) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func (f *fakeCDP) nextTarget() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.targets++
	return "t" + strconv.Itoa(f.targets)
}

func (f *fakeCDP) currentSession() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.session
}

func (f *fakeCDP) defaults() {
	f.handle("Target.createTarget", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"targetId": f.nextTarget()})
	})
	f.handle("Target.attachToTarget", func(c cdpCommand) {
		var p struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(c.Params, &p)
		session := "s-" + p.TargetID
		f.mu.Lock()
		f.session = session
		f.mu.Unlock()
		f.respond(c.ID, map[string]any{"sessionId": session})
	})
	f.handle("Target.getTargetInfo", func(c cdpCommand) {
		var p struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(c.Params, &p)
		f.respond(c.ID, map[string]any{"targetInfo": map[string]any{"targetId": p.TargetID}})
	})
	f.handle("Target.closeTarget", func(c cdpCommand) {
		f.respond(c.ID, map[string]any{"success": true})
	})
	f.handle("Network.enable", func(c cdpCommand) { f.respond(c.ID, map[string]any{}) })
	f.handle("Network.disable", func(c cdpCommand) { f.respond(c.ID, map[string]any{}) })
	f.handle("Network.setBlockedURLs", func(c cdpCommand) { f.respond(c.ID, map[string]any{}) })
}

func (f *fakeCDP) dial(t *testing.T, onEvent func(cdpEvent)) *cdpConn {
	t.Helper()
	conn, err := dialCDP(context.Background(), f.wsURL(), 1, onEvent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.close)
	return conn
}

func newFakeBridge(t *testing.T, f *fakeCDP) *Bridge {
	t.Helper()
	b := NewBridge(nil, nil)
	b.connect = func(ctx context.Context) (*cdpConn, error) {
		return dialCDP(ctx, f.wsURL(), 1, b.handleEvent)
	}
	t.Cleanup(func() {
		b.mu.Lock()
		c := b.conn
		b.mu.Unlock()
		if c != nil {
			c.close()
		}
	})
	return b
}

func cdpNumber(v float64) map[string]any {
	return map[string]any{"result": map[string]any{"type": "number", "value": v}}
}

func cdpString(v string) map[string]any {
	return map[string]any{"result": map[string]any{"type": "string", "value": v}}
}

func cdpBool(v bool) map[string]any {
	return map[string]any{"result": map[string]any{"type": "boolean", "value": v}}
}
