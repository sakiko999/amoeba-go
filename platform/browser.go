// 浏览器载具执行面:由 components/service-browser.Router 实现(池化多实例,空闲实例
// 回空页、最近一个常驻),经 ServiceContext.Browser 注入平台(与 JSRunner 同一
// 注入形态)。接口只描述 CDP 语义,不依赖 mygo/WebView2。

package platform

import (
	"context"
	"errors"
	"net/url"
	"time"
)

var (
	// ErrBrowserUnavailable 载具不可用:非 Windows、载具子进程起不来、CDP 连接断开。
	ErrBrowserUnavailable = errors.New("platform: browser unavailable")
	// ErrBrowserTimeout 单条 CDP 命令 30s、Navigate 等文档就绪 20s、
	// WaitUntil 调用方给定 deadline 到期。
	ErrBrowserTimeout = errors.New("platform: browser timeout")
	// ErrBrowserTargetLost target 消失且一次自愈重建仍失败。
	ErrBrowserTargetLost = errors.New("platform: browser target lost")
	// ErrBrowserLoginRequired 平台层判定「需要登录/登录失效」(载具本身不产生)。
	ErrBrowserLoginRequired = errors.New("platform: browser login required")
	// ErrBrowserRiskControl 平台层判定「风控校验页/风控框」(载具本身不产生)。
	ErrBrowserRiskControl = errors.New("platform: browser risk control")
)

// Browser 按 origin 隔离 page target:同 origin 的 Do 串行,跨 origin 并发。
// 实现必须保证 fn 返回前,本 origin 的页面不会被另一个 Do 导航走。
// 池化实现下 origin 映射到平台 profile,同平台任务天然独占一个实例。
type Browser interface {
	// Do 在 origin 对应的 page target 上独占执行 fn。载具实例即开即用,
	// Do 返回后实例先冷却 1 秒;之后最近使用的实例常驻并回到空页,其余回收。
	// ctx 取消时 fn 内的调用尽快返回,排队中的 Do 也响应 ctx。
	// 注意:fn 执行期间持有本 origin 的不可重入锁,禁止在 fn 内对本 origin 调用
	// Do/CloseTab(会永久死锁);需要关闭时在 Do 返回后再调用。
	Do(ctx context.Context, origin string, fn func(BrowserPage) error) error
	// CloseTab 关闭 origin 的 target;池化实现下 target 随任务实例销毁,为空操作。
	CloseTab(ctx context.Context, origin string) error
}

// BrowserPage 是 Do 内的页面句柄。只能在 fn 执行期间使用;fn 返回后句柄失效。
type BrowserPage interface {
	// Navigate 导航到 rawURL 并等新文档 readyState === 'complete'。
	// rawURL 的 origin 必须等于 Do 的 origin,否则报错(防跨 origin 串页)。
	Navigate(ctx context.Context, rawURL string) error
	// Evaluate 在页面当前文档执行 JS:awaitPromise=true、returnByValue=true。
	// 返回 any 的 JSON 形态:bool/float64/string/[]any/map[string]any/nil。
	Evaluate(ctx context.Context, expression string) (any, error)
	// WaitUntil 轮询 condition 直到 === true 或 timeout;导航过渡期错误视为未就绪。
	WaitUntil(ctx context.Context, condition string, timeout time.Duration) error
	// Cookies 读指定 URL 的 cookie(含 HttpOnly):Network.getCookies。
	Cookies(ctx context.Context, rawURL string) (map[string]string, error)
	// CaptureNavigate 强制导航(rawURL,同 origin 校验同 Navigate)并捕获页面自己
	// 发出的、match(u) 为真的文本响应。收集直到「无匹配 inflight 且静默 idle」
	// 或到达 timeout——超时**不报错**,返回已收集的部分(与 TS capturePage 一致)。
	// match 收到解析后的 *url.URL(重定向按最后一跳的 URL 判定)。
	CaptureNavigate(ctx context.Context, rawURL string, match func(*url.URL) bool,
		idle, timeout time.Duration) ([]CapturedResponse, error)
	// Capture 在当前文档执行 trigger,捕获 trigger 之后页面自己发出的匹配文本响应
	// (不导航;对应 TS capturePage 的 cdpEvaluate trigger)。触发后的收集语义与
	// CaptureNavigate 相同:静默 idle 或 timeout 结束,超时不报错。
	Capture(ctx context.Context, trigger func() error, match func(*url.URL) bool,
		idle, timeout time.Duration) ([]CapturedResponse, error)
}

// BrowserCookie 载具 profile 里的一条 cookie(含 HttpOnly)。Domain 保留前导点
// (如 ".weibo.com");只读场景由调用方按域名后缀过滤。
type BrowserCookie struct {
	Name   string
	Value  string
	Domain string
	Path   string
}

// BrowserEvalError 页面内 JS 抛异常,或返回值不可 returnByValue 序列化。
// 用 errors.As 判别;Error() 不带完整表达式(避免把用户关键词打进 UI 日志)。
type BrowserEvalError struct {
	Expression  string
	Description string
}

func (e *BrowserEvalError) Error() string {
	return "browser: evaluate 异常: " + e.Description
}

// CapturedResponse CaptureNavigate 捕获到的页面自发响应(body 已解码为文本)。
type CapturedResponse struct {
	URL  string
	Body string
}
