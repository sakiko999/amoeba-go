// cookie 串解析（浏览器复制格式 `k=v; k2=v2`）。

package bili

import "strings"

// parseCookieDict cookie 串 → 字典。
func parseCookieDict(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		out[strings.TrimSpace(part[:eq])] = strings.TrimSpace(part[eq+1:])
	}
	return out
}

// extractCookie 按 key 取 cookie 值，不存在返回空串。
func extractCookie(cookie, key string) string {
	return parseCookieDict(cookie)[key]
}
