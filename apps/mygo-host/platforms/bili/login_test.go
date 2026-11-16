// 扫码登录协议测试（离线）：generate/poll 状态机、Set-Cookie 解析、
// 成功写 provider、已扫回调只触发一次、过期/无 cookie 错误。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestQRGenerate(t *testing.T) {
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"qrcode_key":"KEY","url":"https://qr.example.com/1"}}`), nil
	}}
	sc := newTestContext(t, fs, testNow)
	qr, err := QRGenerate(context.Background(), sc)
	if err != nil {
		t.Fatal(err)
	}
	if qr.Key != "KEY" || qr.URL != "https://qr.example.com/1" {
		t.Errorf("qr = %+v", qr)
	}
	req := fs.find("/web/qrcode/generate")
	if req.Header.Get("user-agent") != loginUA || req.Header.Get("referer") != "https://www.bilibili.com/" {
		t.Errorf("headers = %v", req.Header)
	}

	fail := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":-1,"message":"boom"}`), nil
	}}
	_, err = QRGenerate(context.Background(), newTestContext(t, fail, testNow))
	if err == nil || err.Error() != "bili 扫码: generate 失败(boom)" {
		t.Fatalf("generate 失败 err = %v", err)
	}
}

func TestQRPollStatuses(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		cookie []string
		status int
	}{
		{"pending", `{"data":{"code":86101}}`, nil, QRStatusPending},
		{"scanned", `{"data":{"code":86090}}`, nil, QRStatusScanned},
		{"expired", `{"data":{"code":86038}}`, nil, QRStatusExpired},
		{"no-data", `{}`, nil, -1},
		{"success", `{"data":{"code":0,"refresh_token":"RT"}}`,
			[]string{"SESSDATA=abc; Path=/; Domain=.bilibili.com", "bili_jct=def; Path=/"}, QRStatusSuccess},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
				return cookieResponse(http.StatusOK, c.body, c.cookie), nil
			}}
			poll, err := QRPoll(context.Background(), newTestContext(t, fs, testNow), "KEY")
			if err != nil {
				t.Fatal(err)
			}
			if poll.Status != c.status {
				t.Errorf("status = %d, want %d", poll.Status, c.status)
			}
			mustContain(t, fs.find("qrcode_key=KEY").URL, "qrcode_key=KEY", "轮询参数")
			if c.status == QRStatusSuccess {
				if poll.Cookie != "SESSDATA=abc; bili_jct=def" || poll.RefreshToken != "RT" {
					t.Errorf("cookie/refresh = %q/%q", poll.Cookie, poll.RefreshToken)
				}
			} else if poll.Cookie != "" {
				t.Errorf("非成功不应带 cookie: %q", poll.Cookie)
			}
		})
	}
}

func TestScanLoginSuccessWritesProvider(t *testing.T) {
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return cookieResponse(http.StatusOK, `{"data":{"code":0}}`, []string{"SESSDATA=abc; Path=/"}), nil
	}}
	sc := newTestContext(t, fs, testNow)
	qrURLs := []string{}
	result, err := ScanLogin(context.Background(), sc, ScanLoginOptions{
		QR:   &QrCode{Key: "KEY", URL: "https://qr"},
		OnQR: func(url string) { qrURLs = append(qrURLs, url) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Cookie != "SESSDATA=abc" {
		t.Errorf("result = %+v", result)
	}
	if len(qrURLs) != 1 || qrURLs[0] != "" {
		t.Errorf("成功应清空二维码: %v", qrURLs)
	}
	provider, ok := sc.Creds.Lookup("bili")
	if !ok || provider != "SESSDATA=abc" {
		t.Errorf("provider = %q/%v", provider, ok)
	}
}

func TestScanLoginScannedCallbackOnce(t *testing.T) {
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"code":86090}}`), nil
	}}
	sc := newTestContext(t, fs, testNow)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scanned := 0
	_, err := ScanLogin(ctx, sc, ScanLoginOptions{
		QR:        &QrCode{Key: "KEY"},
		OnScanned: func() { scanned++; cancel() },
	})
	if err == nil || err.Error() != "扫码登录已取消" {
		t.Fatalf("err = %v", err)
	}
	if scanned != 1 {
		t.Errorf("OnScanned 次数 = %d", scanned)
	}
}

func TestScanLoginExpired(t *testing.T) {
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"code":86038}}`), nil
	}}
	_, err := ScanLogin(context.Background(), newTestContext(t, fs, testNow), ScanLoginOptions{QR: &QrCode{Key: "KEY"}})
	if err == nil || err.Error() != "bili 扫码: 二维码已过期" {
		t.Fatalf("err = %v", err)
	}
}

func TestScanLoginSuccessWithoutCookie(t *testing.T) {
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":{"code":0}}`), nil
	}}
	_, err := ScanLogin(context.Background(), newTestContext(t, fs, testNow), ScanLoginOptions{QR: &QrCode{Key: "KEY"}})
	if err == nil || !strings.Contains(err.Error(), "成功但未取到 cookie") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseSetCookie(t *testing.T) {
	raw := "SESSDATA=abc; Path=/; HttpOnly\nbili_jct=def; Path=/\nbroken"
	if got := parseSetCookie(raw); got != "SESSDATA=abc; bili_jct=def" {
		t.Errorf("parseSetCookie = %q", got)
	}
}

func TestScanLoginGeneratesQR(t *testing.T) {
	polls := 0
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/generate") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"qrcode_key":"K2","url":"https://qr/2"}}`), nil
		}
		polls++
		return cookieResponse(http.StatusOK, `{"data":{"code":0}}`, []string{"SESSDATA=xyz; Path=/"}), nil
	}}
	sc := newTestContext(t, fs, testNow)
	var seen []string
	result, err := ScanLogin(context.Background(), sc, ScanLoginOptions{OnQR: func(url string) { seen = append(seen, url) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "https://qr/2" || seen[1] != "" {
		t.Errorf("OnQR 序列 = %v", seen)
	}
	if result.Cookie != "SESSDATA=xyz" || polls != 1 {
		t.Errorf("result/polls = %+v/%d", result, polls)
	}
	if fs.find("qrcode_key=K2") == nil {
		t.Error("轮询应使用新二维码 key")
	}
}
