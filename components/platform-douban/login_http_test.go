// 豆瓣纯 HTTP 扫码：generate 返回远程图，宿主转 data URL；status=login 时
// Set-Cookie 的 dbcl2 回灌载具。

package douban

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform/login"
	"mygo-host/platform/login/logintest"
)

func TestLoginFlowHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/j/mobile/login/qrlogin_code", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "bid=b1; Domain=.douban.com; Path=/")
		_, _ = fmt.Fprint(w, `{"status":"success","message":"success","payload":{"code":"C1","login_url":"https://accounts.douban.com/passport/qrlogin?code=C1","img":"https://img.example/qr.png"}}`)
	})
	mux.HandleFunc("/j/mobile/login/qrlogin_status", func(w http.ResponseWriter, req *http.Request) {
		if !strings.Contains(req.Header.Get("Cookie"), "bid=b1") {
			t.Errorf("status 未携带 generate 会话 cookie: %q", req.Header.Get("Cookie"))
		}
		w.Header().Add("Set-Cookie", "dbcl2=42:abc; Domain=.douban.com; Path=/")
		_, _ = fmt.Fprint(w, `{"status":"success","payload":{"login_status":"login"}}`)
	})
	mux.HandleFunc("/qr.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	})

	b := &logintest.Browser{}
	env, srv := logintest.Env(mux, b)
	t.Cleanup(srv.Close)
	sess := &login.Session{}
	qr, qrText, err := doubanHTTPStart(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(qr, "data:image/png;base64,") || qrText == "" {
		t.Fatalf("start = %q / %q", qr, qrText)
	}
	done, cookies, user, err := doubanHTTPPoll(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if !done || user != "42" || cookies["dbcl2"] != "42:abc" {
		t.Fatalf("poll = done=%v user=%q cookies=%v", done, user, cookies)
	}
	injected := false
	for _, c := range b.Injected {
		if c.Name == "dbcl2" && c.Domain == ".douban.com" {
			injected = true
		}
	}
	if !injected {
		t.Fatalf("cookie 未回灌载具: %+v", b.Injected)
	}
}
