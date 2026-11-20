// douyin cookie:登录态(sc.Creds)优先,否则匿名 ttwid warmup(进程内 memo);
// warmup 失败或空 Set-Cookie 回落 DEFAULT_TTWID(缺 ttwid 时 enter 直接 415
// 或 200 空 body)。

package douyin

import (
	"context"
	"strings"
	"sync"

	"mygo-host/platform"
)

// cookieJarState 匿名 ttwid 的进程内 memo(并发首个调用者负责 warmup)。
type cookieJarState struct {
	mu   sync.Mutex
	done bool
	jar  string
}

var anonCookie cookieJarState

// douyinCookie 取当前 cookie:平台登录 cookie 优先,否则匿名 ttwid。
func douyinCookie(ctx context.Context, sc *platform.ServiceContext) string {
	if sc.Creds != nil {
		if v, ok := sc.Creds.Lookup("douyin"); ok && v != "" {
			return v
		}
	}
	return anonCookie.load(func() string { return warmupTTWID(ctx, sc) })
}

// load 只执行一次 warmup;空结果回落 DEFAULT_TTWID。
func (c *cookieJarState) load(warmup func() string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.done {
		c.jar = warmup()
		if c.jar == "" {
			c.jar = defaultTTWID
		}
		c.done = true
	}
	return c.jar
}

// warmupTTWID GET live.douyin.com 首页并解析 Set-Cookie;失败返回空。
func warmupTTWID(ctx context.Context, sc *platform.ServiceContext) string {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:     liveBase + "/",
		Headers: map[string]string{"user-agent": uaEnter},
	})
	if err != nil {
		return ""
	}
	return parseSetCookie(res.Headers["set-cookie"])
}

// parseSetCookie 多值 "\n" 分隔 → 只留 name=value,"; " 连接(TS warmup 原文)。
func parseSetCookie(raw string) string {
	parts := []string{}
	for _, line := range strings.Split(raw, "\n") {
		pair := strings.TrimSpace(strings.Split(line, ";")[0])
		if strings.Contains(pair, "=") {
			parts = append(parts, pair)
		}
	}
	return strings.Join(parts, "; ")
}
