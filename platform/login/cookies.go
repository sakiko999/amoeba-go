// 扫码登录的 cookie 工具：镜像串解析、Set-Cookie 合并与按域名归一。

package login

import (
	"sort"
	"strings"

	"mygo-host/platform"
)

// ParseCookiePairs "k=v; k2=v2" → map。
func ParseCookiePairs(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		kv := strings.TrimSpace(part)
		if i := strings.IndexByte(kv, '='); i > 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}

// MergeSetCookies 把 Set-Cookie 响应头（多值以 "\n" 连接）合并进 name→value。
func MergeSetCookies(dst map[string]string, raw string) {
	if dst == nil || raw == "" {
		return
	}
	for _, line := range strings.Split(raw, "\n") {
		first, _, _ := strings.Cut(strings.TrimSpace(line), ";")
		name, value, ok := strings.Cut(first, "=")
		if ok && name != "" {
			dst[name] = value
		}
	}
}

// CookieHeader 把 name→value 序列化为 Cookie 请求头。
func CookieHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	return strings.Join(parts, "; ")
}

// CookiesFor 把全量 cookie 归一到这些域名下的 name→value（同名后写覆盖）。
func CookiesFor(all []platform.BrowserCookie, hosts []string) map[string]string {
	out := map[string]string{}
	for _, c := range all {
		if !CookieDomainMatch(c.Domain, hosts) || c.Value == "" {
			continue
		}
		out[c.Name] = c.Value
	}
	return out
}

func CookieDomainMatch(domain string, hosts []string) bool {
	d := strings.ToLower(strings.TrimPrefix(domain, "."))
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimPrefix(h, "."))
		if h == "" {
			continue
		}
		if d == h || strings.HasSuffix(d, "."+h) {
			return true
		}
	}
	return false
}

// SerializeCookies 排序后拼 "k=v; k2=v2"，作为 credentials.json 的 HTTP 镜像。
func SerializeCookies(cookies map[string]string) string {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	return strings.Join(parts, "; ")
}
