// twitch 频道 login / 分类 slug 归一化(playback 路由与渠道输入共用):
// 接受裸值、@前缀与 twitch.tv 频道域链接;host、字符集与一级路径保留字
// 只认官方规则,避免把 clip/VOD 等站内路径误归一成频道或分类。

package twitch

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	loginRe      = regexp.MustCompile(`^[a-z0-9_]{3,25}$`)
	categoryRe   = regexp.MustCompile(`^[a-z0-9-]{2,64}$`)
	schemeRe     = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)
	twitchDotTV  = regexp.MustCompile(`(?i)twitch\.tv`)
	channelHosts = map[string]bool{"twitch.tv": true, "www.twitch.tv": true, "m.twitch.tv": true}
	reserved     = map[string]bool{
		"about": true, "activate": true, "clips": true, "directory": true,
		"downloads": true, "drops": true, "embed": true, "following": true,
		"friends": true, "inventory": true, "jobs": true, "legal": true,
		"login": true, "messages": true, "oauth2": true, "p": true,
		"popout": true, "privacy": true, "search": true, "security": true,
		"settings": true, "signup": true, "store": true, "subscriptions": true,
		"turbo": true, "verify": true, "videos": true, "wallet": true,
	}
)

// isTwitchChannelHost host 是否为频道页所在域(大小写不敏感;clips./dashboard. 等不算)。
func isTwitchChannelHost(host string) bool {
	return channelHosts[strings.ToLower(strings.TrimSpace(host))]
}

// isTwitchLogin 裸字符串是否为可作频道名的 login(不做 URL 解析)。
func isTwitchLogin(value string) bool {
	login := strings.ToLower(strings.TrimSpace(value))
	return loginRe.MatchString(login) && !reserved[login]
}

// normalizeTwitchLogin 裸 login / @login / 频道 URL → 小写 login;非法返回 false。
func normalizeTwitchLogin(input string) (string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	var login string
	switch {
	case strings.HasPrefix(raw, "@"):
		login = strings.ToLower(raw[1:])
	case strings.Contains(raw, "/") || twitchDotTV.MatchString(raw):
		login = loginFromURL(raw)
	default:
		login = strings.ToLower(raw)
	}
	if login == "" || !loginRe.MatchString(login) || reserved[login] {
		return "", false
	}
	return login, true
}

// loginFromURL 只认频道域 + 单段路径(多段路径留给后续路由)。
func loginFromURL(input string) string {
	u, err := parseTwitchURL(input)
	if err != nil || !isTwitchChannelHost(u.Hostname()) {
		return ""
	}
	segments := pathSegments(u)
	if len(segments) != 1 {
		return ""
	}
	login, err := url.PathUnescape(segments[0])
	if err != nil {
		return ""
	}
	return strings.ToLower(login)
}

// normalizeTwitchCategory 裸 slug / 目录页链接 → 小写 slug;非法返回 false。
func normalizeTwitchCategory(input string) (string, bool) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", false
	}
	var slug string
	if strings.Contains(raw, "/") || twitchDotTV.MatchString(raw) {
		slug = categoryFromURL(raw)
	} else {
		slug = strings.ToLower(raw)
	}
	if slug == "" || !categoryRe.MatchString(slug) {
		return "", false
	}
	return slug, true
}

// categoryFromURL 只认频道域下的 `/directory/category/<slug>`。
func categoryFromURL(input string) string {
	u, err := parseTwitchURL(input)
	if err != nil || !isTwitchChannelHost(u.Hostname()) {
		return ""
	}
	segments := pathSegments(u)
	if len(segments) != 3 {
		return ""
	}
	if !strings.EqualFold(segments[0], "directory") || !strings.EqualFold(segments[1], "category") {
		return ""
	}
	slug, err := url.PathUnescape(segments[2])
	if err != nil {
		return ""
	}
	return strings.ToLower(slug)
}

// parseTwitchURL 允许 `twitch.tv/xxx` 这种无 scheme 写法。
func parseTwitchURL(input string) (*url.URL, error) {
	if !schemeRe.MatchString(input) {
		input = "https://" + input
	}
	return url.Parse(input)
}

// pathSegments 按编码后的路径切段(保留 %xx,由调用方按段解码,与 TS 语义一致)。
func pathSegments(u *url.URL) []string {
	parts := strings.Split(u.EscapedPath(), "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}
