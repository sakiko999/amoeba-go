// jsrunner —— 签名 JS 的执行面:宿主内一个隐藏 WebView 窗口,Page.Eval 执行
// 平台签名 blob(douyu CryptoJS+crptext、youtube n 参数解密)。签名代码维持
// JS 原文执行是设计决策(服务端动态下发/混淆轮换,移植成 Go 必坏)。
//
// 窗口必须在 app ready 后创建(宿主 WhenReady 里调 Init);Eval 等页面就绪。
// 未 Init(如 go test)时 Call 返回 ErrJSRunnerUnavailable。

package jsrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"mygo-host/platform"
)

// WebView 用隐藏窗口的 Page.Eval 执行签名 JS。
type WebView struct {
	mu     sync.Mutex
	ready  chan struct{}
	page   *mygo.Page
	failed error
}

// New 建 runner(未就绪状态;Init 之后可用)。
func New() *WebView {
	return &WebView{ready: make(chan struct{})}
}

// Init 在 app ready 后创建隐藏窗口并等待页面就绪。只能调用一次;
// 窗口创建失败时 Call 永远返回该错误(签名平台降级,不影响其他能力)。
func (r *WebView) Init() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.ready:
		return
	default:
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:  "js-runner",
		URL:    "about:blank",
		Hidden: true,
		Width:  1,
		Height: 1,
	})
	page := win.Page()
	page.OnDOMReady(func() {
		r.page = page
		close(r.ready)
	})
	// 窗口已 Hidden,不会再闪;失败兜底由 Call 的等待超时承接。
	go func() {
		select {
		case <-r.ready:
		case <-time.After(30 * time.Second):
			r.mu.Lock()
			if r.page == nil {
				r.failed = fmt.Errorf("js runner: 页面 30s 未就绪")
				close(r.ready)
			}
			r.mu.Unlock()
		}
	}()
}

// Call 实现 platform.JSRunner:等价 TS `new Function(code + return fn(...args))`。
func (r *WebView) Call(ctx context.Context, code, fn string, args []any) (any, error) {
	select {
	case <-r.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.mu.Lock()
	page, failed := r.page, r.failed
	r.mu.Unlock()
	if failed != nil || page == nil {
		return nil, platform.ErrJSRunnerUnavailable
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	script := fmt.Sprintf("(function(){%s\n;return %s(...%s)})()", code, fn, argsJSON)
	return page.EvalContext(ctx, script)
}
