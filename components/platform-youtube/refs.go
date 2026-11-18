// YouTube 参数归一化(纯函数):youtube:playlist 接受裸 playlistId、
// playlist/watch/youtu.be 链接;RD/WL/LL/LM 前缀(动态/私有/需登录)一律拒收。

package youtube

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	playlistIDRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{10,80}$`)
	youtubeHintRe   = regexp.MustCompile(`(?i)youtu\.?be|youtube\.com`)
	urlSchemeRe     = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)
	rejectPlaylists = []string{"RD", "WL", "LL", "LM"}
	youtubeHosts    = map[string]bool{
		"youtube.com": true, "www.youtube.com": true, "m.youtube.com": true,
		"music.youtube.com": true, "youtu.be": true,
	}
)

// normalizeYoutubePlaylistID 归一化播放列表 ID;非法/非 YouTube host/私有列表返回 ""。
func normalizeYoutubePlaylistID(input string) string {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ""
	}
	id := raw
	if strings.Contains(raw, "/") || youtubeHintRe.MatchString(raw) {
		parsed := parseYouTubeURL(raw)
		if parsed == nil || !youtubeHosts[strings.ToLower(parsed.Hostname())] {
			return ""
		}
		id = parsed.Query().Get("list")
	}
	if !playlistIDRe.MatchString(id) {
		return ""
	}
	for _, prefix := range rejectPlaylists {
		if strings.HasPrefix(id, prefix) {
			return ""
		}
	}
	return id
}

func parseYouTubeURL(input string) *url.URL {
	candidate := input
	if !urlSchemeRe.MatchString(candidate) {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return nil
	}
	return parsed
}
