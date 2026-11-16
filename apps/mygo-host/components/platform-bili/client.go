// BilibiliClient 的 Go 版：HTTP 统一入口（cookie 三档 / buvid 补齐 / code 校验）
// 与模块级共享状态（nav 按 cookie 分 key、匿名 buvid、w_webid、合集/系列 memo）。
// 两套签名语义分开（web 端 signWeb / 直播端 signLiveParams），见 wbi.go。

package bili

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"mygo-host/platform"
)

const (
	// bilibiliUA 对齐 platform-kit 的 DESKTOP_CHROME_UA（反爬按整串匹配，勿改）。
	bilibiliUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	apiMain        = "https://api.bilibili.com"
	defaultReferer = "https://www.bilibili.com/"
	liveBase       = "https://live.bilibili.com"
)

// client 平台单例（每个 New 一份；对应 TS 模块级单例）。
type client struct {
	sc *platform.ServiceContext

	mu        sync.Mutex
	navs      map[string]*navEntry
	buvid     string
	buvidSet  bool
	buvidCall *buvidCall

	webIDMu sync.Mutex
	webID   string
	webIDAt int64

	seasonMemo memo
	seriesMemo memo
}

func newClient(sc *platform.ServiceContext) *client {
	return &client{sc: sc, navs: map[string]*navEntry{}}
}

func (c *client) now() int64 { return c.sc.NowMs() }

func (c *client) log(format string, args ...any) { c.sc.Log(format, args...) }

// resolveCookie 对齐 resolveCookie(explicit, "bili")：nil → provider；空串 → 强制匿名。
func (c *client) resolveCookie(explicit *string) string {
	v, _ := c.sc.Creds.ResolveCookie(explicit, "bili")
	return v
}

// reqOptions getJson 选项（allowCodeError / buvid / referer / cookie）。
type reqOptions struct {
	Cookie         *string
	Referer        string
	Buvid          bool
	AllowCodeError bool
}

// getJSON 统一 GET JSON：UA/referer/cookie 三档/buvid 补齐/code 校验。
// 返回原始 body 与解码后的对象；code!=0 且未允许时返回错误。
func (c *client) getJSON(ctx context.Context, url string, opts reqOptions) (platform.RawJSON, map[string]any, error) {
	headers := map[string]string{
		"user-agent": bilibiliUA,
		"referer":    opts.Referer,
	}
	if headers["referer"] == "" {
		headers["referer"] = defaultReferer
	}
	if cookie := c.resolveCookie(opts.Cookie); cookie != "" {
		headers["cookie"] = cookie
	}
	if opts.Buvid {
		if err := c.ensureBuvid(ctx); err != nil {
			return nil, nil, err
		}
		anon := c.anonBuvid()
		if anon != "" && !hasBuvid3(headers["cookie"]) {
			if cur := headers["cookie"]; cur != "" {
				headers["cookie"] = cur + "; " + anon
			} else {
				headers["cookie"] = anon
			}
		}
	}
	raw, err := c.sc.HTTP.GetJSON(ctx, url, headers)
	if err != nil {
		return nil, nil, err
	}
	var data map[string]any
	if err := platform.UnmarshalJSON(raw, &data); err != nil {
		return raw, nil, err
	}
	if n := jnum(data["code"]); n != 0 && !opts.AllowCodeError {
		message := platform.JSString(data["message"])
		if message == "" {
			message = "unknown error"
		}
		return nil, nil, fmt.Errorf("bilibili API %s: %s", jsNum(n), message)
	}
	return raw, data, nil
}

// hasBuvid3 判断 cookie 串是否已带 buvid3（避免重复追加匿名指纹）。
func hasBuvid3(cookie string) bool {
	for _, part := range strings.Split(cookie, ";") {
		if strings.HasPrefix(strings.TrimSpace(part), "buvid3=") {
			return true
		}
	}
	return false
}

// cookiePtr info.cookie 未传 → nil（走 provider）；显式空串 → 强制匿名。
func cookiePtr(info platform.SourceInfo) *string {
	if v, ok := info["cookie"]; ok {
		return &v
	}
	return nil
}

// nonEmptyCookie 对应 `info.cookie || undefined`（空串按未传处理）。
func nonEmptyCookie(info platform.SourceInfo) *string {
	if v := info["cookie"]; v != "" {
		return &v
	}
	return nil
}

// nonEmptyResolveCookie 对应 ResolveInfo 上的同一语义。
func nonEmptyResolveCookie(info platform.ResolveInfo) *string {
	if v := info["cookie"]; v != "" {
		return &v
	}
	return nil
}

// fileStem url 文件名去扩展名（wbi key 提取）。
func fileStem(u string) string {
	parts := strings.Split(u, "/")
	last := parts[len(parts)-1]
	if i := strings.IndexByte(last, '.'); i >= 0 {
		last = last[:i]
	}
	return last
}
