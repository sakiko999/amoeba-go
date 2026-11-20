// 平台扫码登录契约：各 platform-* 组件用 Flow 描述登录判定、取码与轮询，
// service-credentials 只负责会话生命周期、凭据镜像与页面绑定。

package login

import (
	"context"
	"errors"
	"time"

	"mygo-host/platform"
)

// ErrExpired 平台明确回报二维码过期。
var ErrExpired = errors.New("二维码已过期,请点击刷新")

const KeyPrefix = "login:"

// Key 登录会话在载具里的 pin key。
func Key(id string) string { return KeyPrefix + id }

// Browser 是载具桥的凭据侧能力。载具按任务启停：Hold 为登录会话长期占住
// 平台实例，其余方法是一次性任务。
type Browser interface {
	Hold(ctx context.Context, key, origin string) error
	DoAs(ctx context.Context, key, origin string, fn func(platform.BrowserPage) error) error
	CloseTabAs(ctx context.Context, key string) error
	CookiesAs(ctx context.Context, key, origin string) ([]platform.BrowserCookie, error)
	ClearCookies(ctx context.Context, hosts []string) (int, error)
	SetCookies(ctx context.Context, cookies []platform.BrowserCookie) error
}

// Flow 一个平台的扫码登录配方。函数字段为空表示该平台不支持对应路径。
type Flow struct {
	ID          string
	Label       string
	Color       string
	Hosts       []string
	Origin      string
	LoginCookie string
	// BaselineCookie 是扫码前要记录的账号标识（xhs 用 user_id 判断换了账号）。
	BaselineCookie string
	Timeout        time.Duration
	// TrustCookies 启动前直接信 cookie 判定已登录。xhs 游客/失效态也可能
	// 残留 user_id，必须靠登录页页面态判定，故为 false。
	TrustCookies bool
	// Check 覆盖默认 cookie 判定（微博匿名态也有 SUB，必须组合判定）。
	Check func(cookies map[string]string) bool
	// HTTPStart / HTTPPoll 纯 HTTP 扫码，不导航浏览器；HTTPStart 返回二维码。
	HTTPStart func(ctx context.Context, env *Env, sess *Session) (qr, qrText string, err error)
	HTTPPoll  func(ctx context.Context, env *Env, sess *Session) (done bool, cookies map[string]string, user string, err error)
	Open      func(ctx context.Context, env *Env, bp platform.BrowserPage) (qr string, already bool, err error)
	PollCheck func(ctx context.Context, env *Env, sess *Session, cookies map[string]string) (done bool, user string)
	UserHint  func(cookies map[string]string) string
}

func (f Flow) LoggedIn(cookies map[string]string) bool {
	if f.Check != nil {
		return f.Check(cookies)
	}
	return cookies[f.LoginCookie] != ""
}

func (f Flow) User(cookies map[string]string) string {
	if f.UserHint == nil {
		return ""
	}
	return f.UserHint(cookies)
}

func (f Flow) Poll(ctx context.Context, env *Env, sess *Session, cookies map[string]string) (bool, string) {
	if f.PollCheck != nil {
		return f.PollCheck(ctx, env, sess, cookies)
	}
	if f.LoggedIn(cookies) {
		return true, f.User(cookies)
	}
	return false, ""
}

// Session 是一次扫码登录的服务侧状态；Data 留给平台私有的 HTTP 会话。
type Session struct {
	Platform       string
	BaselineUserID string
	Deadline       time.Time
	UsedBrowser    bool
	Data           any
}

// Env 是平台登录配方可用的宿主能力。
type Env struct {
	HTTP    *platform.HostClient
	Browser Browser
	Logf    func(string, ...any)
}

func (e *Env) Log(format string, args ...any) {
	if e.Logf != nil {
		e.Logf(format, args...)
	}
}
