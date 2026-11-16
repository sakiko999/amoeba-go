// bili 扫码登录（纯 HTTP，不依赖浏览器）：generate 拿二维码内容，轮询 poll；
// 成功时 cookie 在 Set-Cookie，refresh_token 在 body。
// 轮询状态码 data.code：86101 未扫码 / 86090 已扫待确认 / 0 成功 / 86038 已过期。
// 导出函数不注册 op（与 TS 侧 scanLogin 不暴露在 ops 面一致）；成功后写 sc.Creds。

package bili

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	passportBase = "https://passport.bilibili.com/x/passport-login/web/qrcode"
	loginUA      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	// pollIntervalMs 协议建议 ~2s。
	pollIntervalMs = 2000
)

// 轮询状态码。
const (
	QRStatusSuccess = 0
	QRStatusExpired = 86038
	QRStatusScanned = 86090
	QRStatusPending = 86101
)

// QrCode 一次二维码申请结果（url 是二维码内容，渲染由 UI 侧 QR 库完成）。
type QrCode struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// QrPoll 一次轮询结果（成功时带 cookie / refreshToken）。
type QrPoll struct {
	Status       int    `json:"status"`
	Cookie       string `json:"cookie,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// LoginResult 登录成功结果。
type LoginResult struct {
	Cookie       string `json:"cookie"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

func loginHeaders() map[string]string {
	return map[string]string{"user-agent": loginUA, "referer": "https://www.bilibili.com/"}
}

// QRGenerate 申请二维码。
func QRGenerate(ctx context.Context, sc *platform.ServiceContext) (QrCode, error) {
	raw, err := sc.HTTP.GetJSON(ctx, passportBase+"/generate", loginHeaders())
	if err != nil {
		return QrCode{}, err
	}
	var resp struct {
		Code    *float64 `json:"code"`
		Message *string  `json:"message"`
		Data    *struct {
			QrcodeKey *string `json:"qrcode_key"`
			URL       *string `json:"url"`
		} `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return QrCode{}, err
	}
	key, url := "", ""
	if resp.Data != nil {
		key, url = deref(resp.Data.QrcodeKey), deref(resp.Data.URL)
	}
	if jnumPtr(resp.Code) != 0 || key == "" || url == "" {
		return QrCode{}, fmt.Errorf("bili 扫码: generate 失败(%s)", derefOr(resp.Message, "无响应"))
	}
	return QrCode{Key: key, URL: url}, nil
}

// QRPoll 轮询一次（需要响应头，故不能走 getJSON）。
func QRPoll(ctx context.Context, sc *platform.ServiceContext, key string) (QrPoll, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:     passportBase + "/poll?qrcode_key=" + encodeURIComponent(key),
		Headers: loginHeaders(),
	})
	if err != nil {
		return QrPoll{}, err
	}
	var body struct {
		Data *struct {
			Code         *float64 `json:"code"`
			RefreshToken *string  `json:"refresh_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(res.Body), &body)
	poll := QrPoll{Status: QRStatusPending}
	if body.Data != nil {
		if body.Data.Code != nil {
			poll.Status = int(*body.Data.Code)
		}
		poll.RefreshToken = deref(body.Data.RefreshToken)
	} else {
		poll.Status = -1
	}
	if poll.Status != QRStatusSuccess {
		return poll, nil
	}
	poll.Cookie = parseSetCookie(res.Headers["set-cookie"])
	return poll, nil
}

// parseSetCookie Set-Cookie 头（多值以 "\n" 连接）→ "k=v; k2=v2"。
func parseSetCookie(raw string) string {
	var parts []string
	for _, line := range strings.Split(raw, "\n") {
		first := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if strings.Contains(first, "=") {
			parts = append(parts, first)
		}
	}
	return strings.Join(parts, "; ")
}

// ScanLoginOptions 完整扫码登录选项。
type ScanLoginOptions struct {
	// TimeoutMs 总超时，默认 3 分钟。
	TimeoutMs int
	// QR 已在别处 generate 过的二维码（必须传回，否则 UI 展示的码永远不会成功）。
	QR *QrCode
	// OnScanned 轮询到「已扫待确认」时回调一次。
	OnScanned func()
	// OnQR 二维码内容推送（"" 表示清除；对应 popular 渠道的 emitQr）。
	OnQR func(url string)
}

// ScanLogin 完整扫码登录：generate（或复用 QR）→ 轮询直到成功/过期/超时/取消。
// 成功后 cookie 写入 sc.Creds（平台默认登录态）。
func ScanLogin(ctx context.Context, sc *platform.ServiceContext, opts ScanLoginOptions) (LoginResult, error) {
	deadline := sc.NowMs() + int64(timeoutOrDefault(opts.TimeoutMs))
	qr := opts.QR
	if qr == nil {
		generated, err := QRGenerate(ctx, sc)
		if err != nil {
			return LoginResult{}, err
		}
		if opts.OnQR != nil {
			opts.OnQR(generated.URL)
		}
		qr = &generated
	}
	scannedNotified := false
	for {
		if ctx.Err() != nil {
			return LoginResult{}, fmt.Errorf("扫码登录已取消")
		}
		if sc.NowMs() > deadline {
			sc.Log("bili 扫码登录超时")
			return LoginResult{}, fmt.Errorf("bili 扫码: 超时未完成")
		}
		poll, err := QRPoll(ctx, sc, qr.Key)
		if err != nil {
			return LoginResult{}, err
		}
		switch poll.Status {
		case QRStatusSuccess:
			if poll.Cookie == "" {
				return LoginResult{}, fmt.Errorf("bili 扫码: 成功但未取到 cookie")
			}
			if err := sc.Creds.Set("bili", poll.Cookie); err != nil {
				return LoginResult{}, err
			}
			sc.Log("bili 扫码登录成功")
			if opts.OnQR != nil {
				opts.OnQR("")
			}
			return LoginResult{Cookie: poll.Cookie, RefreshToken: poll.RefreshToken}, nil
		case QRStatusExpired:
			sc.Log("bili 扫码登录超时")
			return LoginResult{}, fmt.Errorf("bili 扫码: 二维码已过期")
		case QRStatusScanned:
			if !scannedNotified && opts.OnScanned != nil {
				scannedNotified = true
				opts.OnScanned()
			}
		}
		if err := sleepCtx(ctx, pollIntervalMs); err != nil {
			return LoginResult{}, fmt.Errorf("扫码登录已取消")
		}
	}
}

func timeoutOrDefault(ms int) int {
	if ms > 0 {
		return ms
	}
	return 180_000
}

func sleepCtx(ctx context.Context, ms int) error {
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
