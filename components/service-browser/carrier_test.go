//go:build windows

// 端到端:把测试二进制本身当作载具子进程(宿主 main 无法在测试里重执行),
// 走真实 WebView2 CDP:createTarget → attach → evaluate → navigate →
// 无 mygo 桥 → stop/reset。覆盖子进程拉起、端口发现与 profile 清理。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestMain 在子进程模式下直接运行载具,不跑测试用例。
func TestMain(m *testing.M) {
	if os.Getenv("MYGO_BROWSER_CARRIER_CHILD") == "1" {
		RunCarrier()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, _, err := NewService(t.TempDir(), t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	exe := os.Args[0]
	svc.childCommand = func() (*exec.Cmd, error) {
		cmd := exec.Command(exe, "-test.run=^$")
		cmd.Env = append(os.Environ(), "MYGO_BROWSER_CARRIER_CHILD=1")
		return cmd, nil
	}
	return svc
}

func TestCarrierEndToEnd(t *testing.T) {
	svc := newTestService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 残留的 DevToolsActivePort 指向旧端口时必须被 startLocked 删除,
	// 否则 waitPort 会把旧端口当成本次进程。
	staleDir := filepath.Join(svc.profile, "EBWebView")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(staleDir, "DevToolsActivePort")
	if err := os.WriteFile(stalePath, []byte("1\n/devtools/browser/stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	ep, err := svc.Endpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ep.Port <= 0 || ep.WS == "" {
		t.Fatalf("endpoint = %+v", ep)
	}
	if ep.Port == 1 {
		t.Fatal("Endpoint 读到了残留的 DevToolsActivePort")
	}
	first, err := svc.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var version struct {
		Browser string `json:"Browser"`
		WS      string `json:"webSocketDebuggerUrl"`
	}
	if err := getJSON(ctx, ep.HTTP+"/json/version", &version); err != nil {
		t.Fatal(err)
	}
	if version.Browser == "" || version.WS == "" {
		t.Fatalf("version = %+v", version)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<title>carrier-e2e</title><script>window.__probe = 42</script>`)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(ep.WS, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	caller := &cdpCaller{conn: conn}

	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := caller.call("Target.createTarget", map[string]any{"url": "about:blank"}, "", &created); err != nil {
		t.Fatal(err)
	}
	if created.TargetID == "" {
		t.Fatal("createTarget 未返回 targetId")
	}

	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := caller.call("Target.attachToTarget",
		map[string]any{"targetId": created.TargetID, "flatten": true}, "", &attached); err != nil {
		t.Fatal(err)
	}
	if attached.SessionID == "" {
		t.Fatal("attachToTarget 未返回 sessionId")
	}

	if got := evalNumber(t, caller, attached.SessionID, "1+1"); got != 2 {
		t.Fatalf("1+1 = %v", got)
	}
	if err := caller.call("Page.navigate",
		map[string]any{"url": server.URL}, attached.SessionID, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		return evalString(t, caller, attached.SessionID, "document.title") == "carrier-e2e"
	})
	if got := evalString(t, caller, attached.SessionID, "typeof window.mygo"); got != "undefined" {
		t.Fatalf("载具页面不应有 mygo 桥, got %q", got)
	}
	if err := caller.call("Target.closeTarget",
		map[string]any{"targetId": created.TargetID}, "", nil); err != nil {
		t.Fatal(err)
	}

	again, err := svc.Endpoint(ctx)
	if err != nil || again.Port != ep.Port {
		t.Fatalf("重复 endpoint 应复用同一进程: %+v, %v", again, err)
	}
	count, err := svc.TargetCount(ctx)
	if err != nil || count < 1 {
		t.Fatalf("targetCount = %d, %v", count, err)
	}

	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	if svc.Status().Running {
		t.Fatal("Stop 后仍在运行")
	}
	restarted, err := svc.snapshot(ctx)
	if err != nil {
		t.Fatalf("重启: %v", err)
	}
	if restarted.gen <= first.gen {
		t.Fatalf("重启后 gen = %d, want > %d", restarted.gen, first.gen)
	}
	if restarted.Port == ep.Port {
		t.Fatalf("重启后端口未变: %d", restarted.Port)
	}
	if err := svc.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(svc.profile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Reset 后 profile 仍存在: %v", err)
	}
}

type cdpCaller struct {
	conn *websocket.Conn

	mu sync.Mutex
	id int
}

func (c *cdpCaller) call(method string, params any, sessionID string, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id++
	req := map[string]any{"id": c.id, "method": method}
	if params != nil {
		req["params"] = params
	}
	if sessionID != "" {
		req["sessionId"] = sessionID
	}
	if err := c.conn.WriteJSON(req); err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := c.conn.ReadJSON(&msg); err != nil {
			return err
		}
		if msg.ID != c.id {
			continue // 事件
		}
		if msg.Error != nil {
			return errors.New(msg.Error.Message)
		}
		if out != nil && len(msg.Result) > 0 {
			return json.Unmarshal(msg.Result, out)
		}
		return nil
	}
}

type evalResult struct {
	Result struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	} `json:"result"`
	Exception *struct {
		Description string `json:"description"`
	} `json:"exceptionDetails"`
}

func evaluate(t *testing.T, c *cdpCaller, sessionID, expression string) any {
	t.Helper()
	var res evalResult
	if err := c.call("Runtime.evaluate",
		map[string]any{"expression": expression, "returnByValue": true}, sessionID, &res); err != nil {
		t.Fatal(err)
	}
	if res.Exception != nil {
		t.Fatalf("evaluate %q: %s", expression, res.Exception.Description)
	}
	return res.Result.Value
}

func evalNumber(t *testing.T, c *cdpCaller, sessionID, expression string) float64 {
	t.Helper()
	value, ok := evaluate(t, c, sessionID, expression).(float64)
	if !ok {
		t.Fatalf("evaluate %q 不是数字", expression)
	}
	return value
}

func evalString(t *testing.T, c *cdpCaller, sessionID, expression string) string {
	t.Helper()
	value, ok := evaluate(t, c, sessionID, expression).(string)
	if !ok {
		t.Fatalf("evaluate %q 不是字符串", expression)
	}
	return value
}

func waitFor(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}
