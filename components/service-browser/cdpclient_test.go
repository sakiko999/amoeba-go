// cdpConn 单测:命令硬超时、ctx 竞速、错误帧解码与 target 失效判定。

package browser

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestCDPConnCallTimeout(t *testing.T) {
	f := newFakeCDP(t)
	f.handle("Sink.hang", func(cdpCommand) {})
	conn := f.dial(t, nil)
	conn.cmdTimeout = 80 * time.Millisecond

	err := conn.call(context.Background(), "Sink.hang", nil, "", nil)
	if !errors.Is(err, platform.ErrBrowserTimeout) {
		t.Fatalf("err = %v, want ErrBrowserTimeout", err)
	}
	if !strings.Contains(err.Error(), "CDP 命令超时(Sink.hang)") {
		t.Fatalf("err 文案 = %q", err.Error())
	}
}

func TestCDPConnCallHonorsContext(t *testing.T) {
	f := newFakeCDP(t)
	f.handle("Sink.hang", func(cdpCommand) {})
	conn := f.dial(t, nil)
	conn.cmdTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := conn.call(ctx, "Sink.hang", nil, "", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestCDPConnErrorFrames(t *testing.T) {
	f := newFakeCDP(t)
	f.handle("Boom.bad", func(c cdpCommand) { f.fail(c.ID, -32602, "Invalid parameters") })
	f.handle("Boom.notarget", func(c cdpCommand) { f.fail(c.ID, -32000, "No target with given id found") })
	f.handle("Boom.session", func(c cdpCommand) {
		f.fail(c.ID, -32000, "Session closed. Most likely the page has been closed.")
	})
	conn := f.dial(t, nil)

	err := conn.call(context.Background(), "Boom.bad", nil, "", nil)
	if err == nil || !strings.Contains(err.Error(), "CDP error -32602: Invalid parameters") {
		t.Fatalf("普通错误帧 = %v", err)
	}
	if errors.Is(err, errCDPTargetGone) {
		t.Fatal("参数错误不应判定为 target 失效")
	}

	for _, method := range []string{"Boom.notarget", "Boom.session"} {
		if err := conn.call(context.Background(), method, nil, "", nil); !errors.Is(err, errCDPTargetGone) {
			t.Fatalf("%s: err = %v, want errCDPTargetGone", method, err)
		}
	}
}

// 连续多次无回复时连接应被标记关闭,下一次 Do 才会重新探测/重启载具。
func TestCDPConnConsecutiveTimeoutsClose(t *testing.T) {
	f := newFakeCDP(t)
	f.handle("Sink.hang", func(cdpCommand) {})
	conn := f.dial(t, nil)
	conn.cmdTimeout = 40 * time.Millisecond

	for i := 0; i < maxConsecutiveTimeouts; i++ {
		err := conn.call(context.Background(), "Sink.hang", nil, "", nil)
		if !errors.Is(err, platform.ErrBrowserTimeout) {
			t.Fatalf("第 %d 次 err = %v, want ErrBrowserTimeout", i+1, err)
		}
	}
	if !conn.isClosed() {
		t.Fatal("连续超时后连接应被标记关闭")
	}
}

func TestCDPConnClosedCall(t *testing.T) {
	f := newFakeCDP(t)
	conn := f.dial(t, nil)
	conn.close()

	err := conn.call(context.Background(), "Any.method", nil, "", nil)
	if !errors.Is(err, platform.ErrBrowserUnavailable) {
		t.Fatalf("err = %v, want ErrBrowserUnavailable", err)
	}
}
