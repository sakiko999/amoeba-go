// 纯 HTTP 扫码的公共设施：单跳 GET、重定向解析、JSONP 解码、二维码图片
// 抓取与 cookie 回灌。

package login

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"mygo-host/platform"
)

const HTTPTimeout = 15 * time.Second

// GetNoRedirect 单次 GET，不自动跟随；调用方显式处理 3xx/Set-Cookie。
func GetNoRedirect(ctx context.Context, hc *platform.HostClient, target string, headers map[string]string) (*platform.Response, error) {
	return hc.Do(ctx, platform.Request{URL: target, Headers: headers, NoRedirect: true})
}

// ResolveRedirect 把 Location 解析成绝对地址（支持 //host/path 与相对路径）。
func ResolveRedirect(base, loc string) (string, error) {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return "", errors.New("重定向缺少 Location")
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}

// JSONPJSON 取 JSONP 回调里的 JSON 主体；裸 JSON 原样返回。
func JSONPJSON(body string) string {
	s := strings.TrimSpace(body)
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return s
	}
	i := strings.IndexByte(s, '(')
	j := strings.LastIndexByte(s, ')')
	if i < 0 || j <= i {
		return ""
	}
	return strings.TrimSpace(s[i+1 : j])
}

// DecodeJSONP 解码 JSON/JSONP 响应；空主体返回 false。
func DecodeJSONP(body string, out any) (bool, error) {
	raw := JSONPJSON(body)
	if raw == "" {
		return false, nil
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, err
	}
	return true, nil
}

// TicketURLs 从 HTML/JSON 文本中提取带 ticket 的跨域链接（去转义、去重）。
func TicketURLs(body string) []string {
	s := strings.ReplaceAll(body, `\/`, "/")
	s = strings.ReplaceAll(s, "&amp;", "&")
	seen := map[string]bool{}
	var out []string
	for _, field := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '"' || r == '\'' || r == '<' || r == '>' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if !strings.Contains(field, "ticket=") && !strings.Contains(field, "cdurl=") {
			continue
		}
		u := strings.TrimRight(field, ",;)]}")
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		if !strings.HasPrefix(u, "http") || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// FetchImage 宿主侧取二维码字节转 data URL。失败返回空串而不是远程 URL：
// 载具默认拦截图片，前端只拿 data URL；远程 URL 只会渲染成空白框。
func (e *Env) FetchImage(ctx context.Context, src, referer string) string {
	if e.HTTP == nil || src == "" || strings.HasPrefix(src, "data:") {
		return src
	}
	data, ct, err := e.HTTP.GetBytes(ctx, src, map[string]string{"Referer": referer})
	if err != nil || len(data) == 0 {
		return ""
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	if !strings.HasPrefix(ct, "image/") || !validImage(data) {
		return ""
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func validImage(data []byte) bool {
	return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) ||
		bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) ||
		bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) ||
		(bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")))
}

// InjectCookies 把 HTTP 扫码得到的 cookie 回灌载具；失败只记日志，
// 不影响登录成功（HTTP 渠道仍可用 credentials.json 镜像）。
func (e *Env) InjectCookies(ctx context.Context, domain string, cookies map[string]string) {
	list := make([]platform.BrowserCookie, 0, len(cookies))
	for name, value := range cookies {
		if name == "" || value == "" {
			continue
		}
		list = append(list, platform.BrowserCookie{Name: name, Value: value, Domain: domain, Path: "/"})
	}
	if err := e.Browser.SetCookies(ctx, list); err != nil {
		e.Log("credentials: 回灌 %s cookie 失败: %v", domain, err)
	}
}

// Fallback 空串取默认值。
func Fallback(v, def string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return def
}
