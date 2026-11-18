// 签名 JS 执行面:平台签名 blob(douyu CryptoJS+crptext、youtube n 参数解密)
// 维持 JS 原文执行,不移植成 Go(服务端动态下发/混淆轮换,移植必坏)。
// 宿主形态用隐藏 WebView 的 Page.Eval 实现(jsrunner 包);测试注入 fake。

package platform

import (
	"context"
	"errors"
)

// ErrJSRunnerUnavailable 未配置 JS 执行面(宿主未就绪或环境不支持)。
var ErrJSRunnerUnavailable = errors.New("platform: js runner unavailable")

// JSRunner 执行「code 定义函数 fn,以 args 调用并返回结果」。
// 与 TS nodeJsBackend.call(code, fn, args) 同语义:
// 等价于 new Function(`${code}\n;return ${fn}(...arguments)`)(...args)。
type JSRunner interface {
	Call(ctx context.Context, code, fn string, args []any) (any, error)
}

// JSRunnerFunc 函数适配器。
type JSRunnerFunc func(ctx context.Context, code, fn string, args []any) (any, error)

func (f JSRunnerFunc) Call(ctx context.Context, code, fn string, args []any) (any, error) {
	return f(ctx, code, fn, args)
}
