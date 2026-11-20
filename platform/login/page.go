// 扫码登录页的脚本工具：表达式求值、等待与文档切换守卫。

package login

import (
	"context"
	"time"

	"mygo-host/platform"
)

const DocTimeOriginExpr = `String(performance.timeOrigin)`

// EvalString 执行表达式并归一为字符串（异常/非字符串 → 空）。
func EvalString(bp platform.BrowserPage, ctx context.Context, expr string) string {
	v, err := bp.Evaluate(ctx, expr)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// EvalBool 只认严格 true；读取失败或 null 返回 false（调用方可用 ok 区分）。
func EvalBool(bp platform.BrowserPage, ctx context.Context, expr string) (bool, bool) {
	v, err := bp.Evaluate(ctx, expr)
	if err != nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// SleepCtx 等待 d 或 ctx 取消；返回 false 表示 ctx 已结束。
func SleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// QuickPollDelay 出码前几轮用短间隔，之后进入 400ms 稳态。
func QuickPollDelay(attempt int) time.Duration {
	switch {
	case attempt == 0:
		return 50 * time.Millisecond
	case attempt == 1:
		return 100 * time.Millisecond
	case attempt == 2:
		return 200 * time.Millisecond
	default:
		return 400 * time.Millisecond
	}
}

// BeginDocNavigation 记录当前文档的 timeOrigin 再 location.assign。
func BeginDocNavigation(ctx context.Context, bp platform.BrowserPage, target string) string {
	before := EvalString(bp, ctx, DocTimeOriginExpr)
	_, _ = bp.Evaluate(ctx, `location.assign('`+target+`')`)
	return before
}

// DocNavigated 只有 timeOrigin 真正变化才返回真；过渡期读失败按未切换处理。
func DocNavigated(ctx context.Context, bp platform.BrowserPage, before string) bool {
	if before == "" {
		return true
	}
	after := EvalString(bp, ctx, DocTimeOriginExpr)
	if after == "" {
		return false
	}
	return after != before
}
