// douyin 客户端:enter API(ttwid cookie + msToken/ABOGUS 签名)与请求错误语义。
// UA 必须用 TS 原文 QQBrowser(Chrome 会得到 200 空 body);ABOGUS_JS 维持
// JS 原文执行(sc.JS),不移植成 Go。

package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"mygo-host/platform"
)

const (
	// uaEnter enter/资料接口固定 UA(TS UA_ENTER,反爬按整串匹配)。
	uaEnter = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.5845.97 Safari/537.36 Core/1.116.567.400 QQBrowser/19.7.6764.400"
	// defaultTTWID warmup 失败兜底(缺 ttwid 时 enter 直接 415 或 200 空 body)。
	defaultTTWID = "ttwid=1%7CB1qls3GdnZhUov9o2NxOMxxYS2ff6OSvEWbv0ytbES4%7C1680522049%7C280d802d6d478e3e78d0c807f7c487e7ffec0ae4e5fdd6a0fe74c3c6af149511"
	// desktopChromeUA iesdouyin 匿名资料接口 UA(TS DESKTOP_CHROME_UA)。
	desktopChromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	// liveOrigin 直播站固定 origin;请求地址用可覆写的 liveBase。
	liveOrigin = "https://live.douyin.com"
)

// liveBase / userInfoAPI 用 var 而非 const:离线测试用 httptest 覆写。
var (
	liveBase    = liveOrigin
	userInfoAPI = "https://www.iesdouyin.com/web/api/v2/user/info/"
)

// signDouyinURL 追加随机 msToken 并经 ABOGUS_JS 计算 a_bogus(TS signDouyinUrl)。
func signDouyinURL(ctx context.Context, sc *platform.ServiceContext, rawURL, ua string) (string, error) {
	if sc.JS == nil {
		return "", platform.ErrJSRunnerUnavailable
	}
	withMs := rawURL + "&msToken=" + randomAlphaNum(107)
	query := ""
	if i := strings.Index(withMs, "?"); i >= 0 {
		query = withMs[i+1:]
	}
	result, err := sc.JS.Call(ctx, abogusJS, "getABogus", []any{query, ua})
	if err != nil {
		return "", err
	}
	return withMs + "&a_bogus=" + encodeURIComponent(platform.JSString(result)), nil
}

// fetchRoom 请求 enter API:先取 cookie(登录态优先),再签名发请求,
// 错误语义对齐 douyinClient.getJson(非 2xx/空 body/status_code)。
func fetchRoom(ctx context.Context, sc *platform.ServiceContext, webRID string) (enterResponse, error) {
	cookie := douyinCookie(ctx, sc)
	requestURL := liveBase + "/webcast/room/web/enter/?" + enterRoomParams(webRID)
	signed, err := signDouyinURL(ctx, sc, requestURL, uaEnter)
	if err != nil {
		return enterResponse{}, err
	}
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL: signed,
		Headers: map[string]string{
			"user-agent": uaEnter,
			"referer":    liveOrigin + "/" + webRID,
			"authority":  "live.douyin.com",
			"cookie":     cookie,
		},
	})
	if err != nil {
		return enterResponse{}, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return enterResponse{}, fmt.Errorf("HTTP %d: %s", res.Status, signed)
	}
	body := strings.TrimSpace(res.Body)
	if body == "" {
		return enterResponse{}, fmt.Errorf("douyin empty body for %s", truncate(signed, 80))
	}
	var data enterResponse
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return enterResponse{}, err
	}
	if err := checkEnterStatus(data.StatusCode, data.Prompts, data.StatusMsg); err != nil {
		return enterResponse{}, err
	}
	return data, nil
}

// enterRoomParams 按 TS 插入顺序拼参数(URLSearchParams 语义,末尾保留 msToken=)。
func enterRoomParams(webRID string) string {
	params := [][2]string{
		{"aid", "6383"},
		{"app_name", "douyin_web"},
		{"live_id", "1"},
		{"device_platform", "web"},
		{"language", "zh-CN"},
		{"browser_language", "zh-CN"},
		{"browser_platform", "Win32"},
		{"browser_name", "Chrome"},
		{"browser_version", "125.0.0.0"},
		{"web_rid", webRID},
		{"msToken", ""},
	}
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(parts, param[0]+"="+formEncode(param[1]))
	}
	return strings.Join(parts, "&")
}

// checkEnterStatus 对齐 getJson:status_code 缺失视为成功,非 0 报内容不可用。
func checkEnterStatus(code any, prompts, statusMsg any) error {
	if code == nil {
		return nil
	}
	if n, ok := platform.JSNumber(code); ok && n == 0 {
		return nil
	}
	msg := platform.JSString(prompts)
	if msg == "" {
		msg = platform.JSString(statusMsg)
	}
	if msg == "" {
		msg = "status_code=" + platform.JSString(code)
	}
	return fmt.Errorf("douyin 内容不可用:%s(code %s)", msg, platform.JSString(code))
}

// randomAlphaNum 生成 n 位 [A-Za-z0-9](TS generateMsToken,非加密 RNG)。
func randomAlphaNum(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = chars[rand.Intn(len(chars))]
	}
	return string(out)
}
