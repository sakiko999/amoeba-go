// 载具 profile 归一:同一平台的任务必须落在同一个 WebView2 profile 上,
// 登录 cookie 才有意义。并发实例不能共享 user data folder(第二个进程会
// 等不到 DevToolsActivePort),所以每个平台一份 profile;平台以外的站点
// 回退到末两级域名,各自独立。

package browser

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// profileHosts 把已知平台域名归到稳定的 profile 键(平台 id),子域自动匹配。
var profileHosts = []struct {
	suffix string
	key    string
}{
	{"weibo.com", "weibo"},
	{"weibo.cn", "weibo"},
	{"xiaohongshu.com", "xhs"},
	{"douyin.com", "douyin"},
	{"douban.com", "douban"},
	{"bilibili.com", "bili"},
	{"b23.tv", "bili"},
}

// ProfileKey 返回 origin 对应的 profile 键。非法 origin 报错。
func ProfileKey(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("browser: 非法 origin %q", origin)
	}
	host := strings.ToLower(u.Hostname())
	for _, m := range profileHosts {
		if host == m.suffix || strings.HasSuffix(host, "."+m.suffix) {
			return m.key, nil
		}
	}
	if net.ParseIP(host) == nil {
		labels := strings.Split(host, ".")
		if len(labels) > 2 {
			host = strings.Join(labels[len(labels)-2:], ".")
		}
	}
	return sanitizeProfile(host), nil
}

// sanitizeProfile 只保留目录安全字符;".." 等异常值回退 default。
func sanitizeProfile(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" || strings.Contains(out, "..") {
		return "default"
	}
	return out
}
