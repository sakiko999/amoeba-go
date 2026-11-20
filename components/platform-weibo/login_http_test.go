// 微博纯 HTTP 扫码：JSONP 取码、扫码链接解析、换票与 cookie 回灌。

package weibo

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
	mux.HandleFunc("/sso/v2/qrcode/image", func(w http.ResponseWriter, req *http.Request) {
		if ck := req.Header.Get("Cookie"); ck != "" {
			t.Errorf("取码不应依赖登录页 cookie: %q", ck)
		}
		_, _ = fmt.Fprint(w, `{"retcode":20000000,"msg":"succ","data":{"qrid":"Q1",`+
			`"image":"https://v2.qr.weibo.cn/inf/gen?data=https%3A%2F%2Fpassport.weibo.cn%2Fscan%3Fqr%3DQ1"}}`)
	})
	mux.HandleFunc("/sso/v2/qrcode/check", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `STK_1({"retcode":20000000,"msg":"succ","data":{"url":"https://login.sina.com.cn/sso/login.php?alt=A1"}})`)
	})
	mux.HandleFunc("/inf/gen", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	})
	mux.HandleFunc("/sso/login.php", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "SUB=sub1; Domain=.sina.com.cn; Path=/")
		w.Header().Add("Set-Cookie", "SUBP=42; Domain=.sina.com.cn; Path=/")
		_, _ = fmt.Fprint(w, `<a href="https://passport.weibo.com/wbsso/login?ticket=T1">go</a>`)
	})
	mux.HandleFunc("/wbsso/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "SUB=sub1; Domain=.weibo.com; Path=/")
		w.Header().Add("Set-Cookie", "SSOLoginState=1; Domain=.weibo.com; Path=/")
		_, _ = fmt.Fprint(w, "ok")
	})

	b := &logintest.Browser{}
	env, srv := logintest.Env(mux, b)
	t.Cleanup(srv.Close)
	sess := &login.Session{}
	qr, qrText, err := weiboHTTPStart(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(qr, "data:image/png;base64,") || qrText != "https://passport.weibo.cn/scan?qr=Q1" {
		t.Fatalf("start = %q / %q", qr, qrText)
	}
	done, cookies, user, err := weiboHTTPPoll(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if !done || user != "42" || cookies["SUB"] != "sub1" {
		t.Fatalf("poll = done=%v user=%q cookies=%v", done, user, cookies)
	}
	injected := false
	for _, c := range b.Injected {
		if c.Name == "SUB" && c.Domain == ".weibo.com" {
			injected = true
		}
	}
	if !injected {
		t.Fatalf("cookie 未回灌载具: %+v", b.Injected)
	}
}

func TestTicketURLs(t *testing.T) {
	body := `{"crossDomainUrlList":["https:\/\/passport.weibo.com\/wbsso\/login?ticket=T1&&amp;ssosavestate=1"]}`
	got := login.TicketURLs(body)
	if len(got) != 1 || !strings.Contains(got[0], "ticket=T1") {
		t.Fatalf("ticketURLs = %v", got)
	}
}
