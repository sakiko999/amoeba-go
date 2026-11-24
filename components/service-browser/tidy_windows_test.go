//go:build windows

// 窗口整理回归测试:载具进程树里不得存在"可见且无 WS_EX_TOOLWINDOW"的
// 顶层窗口(否则会占任务栏按钮)。用真实载具 + 一个 CDP target,轮询等
// tidy 生效;覆盖 PROCESSENTRY32W 布局、EnumWindows、跨进程 SetWindowLongPtr。

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"mygo-host/platform"
)

func TestTidyHidesTargetWindow(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("需要交互式桌面会话：CI runner 无法枚举载具窗口")
	}
	svc := newTestService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bridge := NewBridge(svc, t.Logf)
	t.Cleanup(func() { cleanupBridgeTest(bridge, svc) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<title>tidy-window-probe</title><h1>probe</h1>`)
	}))
	defer server.Close()

	err := bridge.Do(ctx, server.URL, func(platform.BrowserPage) error {
		deadline := time.Now().Add(15 * time.Second)
		for {
			styles := visibleWindowStyles(uint32(os.Getpid()))
			if len(styles) > 0 && allToolWindows(styles) {
				return nil
			}
			if time.Now().After(deadline) {
				if len(styles) == 0 {
					return fmt.Errorf("载具进程树内没有任何可见顶层窗口")
				}
				return fmt.Errorf("窗口仍缺 WS_EX_TOOLWINDOW: %x", styles)
			}
			time.Sleep(250 * time.Millisecond)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// visibleWindowStyles 返回 root 的子孙进程持有的全部可见顶层窗口的扩展样式。
func visibleWindowStyles(root uint32) []uintptr {
	pids := descendantPids(root)
	var styles []uintptr
	cb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		pGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if !pids[pid] {
			return 1
		}
		if vis, _, _ := pIsWindowVisible.Call(hwnd); vis == 0 {
			return 1
		}
		style, _, _ := pGetWindowLongPtr.Call(hwnd, gwlExStyle)
		styles = append(styles, style)
		return 1
	})
	_, _, _ = pEnumWindows.Call(cb, 0)
	return styles
}

func allToolWindows(styles []uintptr) bool {
	for _, s := range styles {
		if s&wsExToolWin == 0 || s&wsExAppWin != 0 {
			return false
		}
	}
	return true
}
