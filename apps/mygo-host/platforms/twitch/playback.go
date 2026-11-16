// twitch 直播播放解析与 URL 路由:PlaybackAccessToken(persisted query)
// → usher master → 档位排序 → Stream[]。master 无 ACAO,必须服务端拉取并只把
// variant URL 交给播放器;错误文案不含 token/signature。

package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"mygo-host/platform"
)

const playbackTokenHash = "ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9"

// usherBase 为变量:测试可用本地 httptest 端点替换。
var (
	usherBase   = "https://usher.ttvnw.net/api/v2/channel/hls"
	twitchRoute = regexp.MustCompile(`(?i)(?:^|[./])twitch\.tv/\S*`)
)

// twitchRoutes 频道页路由:只认频道域 + 单段路径。
func twitchRoutes() []platform.Route {
	return []platform.Route{{
		Re:        twitchRoute,
		Kind:      "live",
		ExtractID: twitchRouteID,
	}}
}

func twitchRouteID(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || !isTwitchChannelHost(u.Hostname()) {
		return "", false
	}
	segments := pathSegments(u)
	if len(segments) != 1 {
		return "", false
	}
	login, err := url.PathUnescape(segments[0])
	if err != nil {
		return "", false
	}
	login = strings.ToLower(login)
	return login, isTwitchLogin(login)
}

// resolveTwitchLivePlay 懒解析直播可播流(login = 频道名)。
func resolveTwitchLivePlay(ctx context.Context, sc *platform.ServiceContext, login string) ([]platform.Stream, error) {
	normalized, ok := normalizeTwitchLogin(login)
	if !ok {
		return nil, fmt.Errorf("twitch: 无效频道名 %s", login)
	}
	token, err := fetchPlaybackToken(ctx, sc, normalized)
	if err != nil {
		return nil, err
	}
	master, err := fetchMaster(ctx, sc, buildMasterURL(normalized, token), normalized)
	if err != nil {
		return nil, err
	}
	return twitchStreamsFromMaster(master)
}

// fetchPlaybackToken 取直播播放 token;地区/版权受限直接报错(重试无用)。
func fetchPlaybackToken(ctx context.Context, sc *platform.ServiceContext, login string) (*gqlPlaybackToken, error) {
	body := map[string]any{
		"operationName": "PlaybackAccessToken",
		"variables": map[string]any{
			"isLive":     true,
			"login":      login,
			"isVod":      false,
			"vodID":      "",
			"platform":   "web",
			"playerType": "site",
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"version": 1, "sha256Hash": playbackTokenHash},
		},
	}
	data, err := twitchGQL[struct {
		StreamPlaybackAccessToken *gqlPlaybackToken `json:"streamPlaybackAccessToken"`
	}](ctx, sc, body, gqlOptions{})
	if err != nil {
		return nil, err
	}
	token := data.StreamPlaybackAccessToken
	if token == nil || deref(token.Signature) == "" || deref(token.Value) == "" {
		return nil, fmt.Errorf("twitch: 无法获取播放 token（频道 %s 可能离线）", login)
	}
	if err := checkTokenValue(login, *token.Value); err != nil {
		return nil, err
	}
	return token, nil
}

func checkTokenValue(login, raw string) error {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("twitch: 播放 token 格式异常")
	}
	value, ok := parsed.(map[string]any)
	if !ok {
		return fmt.Errorf("twitch: 播放 token 格式异常")
	}
	geoblock, _ := value["geoblock_reason"].(string)
	geoblock = strings.TrimSpace(geoblock)
	blackout := value["blackout_enabled"] == true || value["blackout_enabled"] == "true"
	if geoblock == "" && !blackout {
		return nil
	}
	why := "版权限制(blackout)"
	if geoblock != "" {
		why = "地区/版权受限(" + geoblock + ")"
	}
	return fmt.Errorf("twitch: %s 无法播放 —— %s", login, why)
}

// buildMasterURL 拼 usher master(随机 p = 播放会话标识;含 sig/token 不得外泄)。
func buildMasterURL(login string, token *gqlPlaybackToken) string {
	params := url.Values{
		"platform":                   {"web"},
		"p":                          {strconv.Itoa(rand.IntN(1_000_000_000))},
		"allow_source":               {"true"},
		"allow_audio_only":           {"true"},
		"playlist_include_framerate": {"true"},
		"supported_codecs":           {"h264"},
		"sig":                        {deref(token.Signature)},
		"token":                      {deref(token.Value)},
	}
	return usherBase + "/" + login + ".m3u8?" + params.Encode()
}

// fetchMaster 拉 master(HLS 播放入口);错误只给状态码,不带含 token 的 URL。
func fetchMaster(ctx context.Context, sc *platform.ServiceContext, masterURL, login string) (string, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{URL: masterURL, Method: "GET"})
	if err != nil {
		return "", fmt.Errorf("twitch: 频道 %s 播放列表请求失败(网络错误)", login)
	}
	if res.Status == 404 {
		return "", &platform.NotFoundError{
			Message: fmt.Sprintf("twitch: 频道 %s 未开播或播放列表不可用(HTTP 404)", login),
		}
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", fmt.Errorf("twitch: 频道 %s 未开播或播放列表不可用(HTTP %d)", login, res.Status)
	}
	return res.Body, nil
}
