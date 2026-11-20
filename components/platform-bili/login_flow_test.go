// B 站纯 HTTP 扫码：generate → poll → 回灌载具。

package bili

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"mygo-host/platform/login"
	"mygo-host/platform/login/logintest"
)

func TestLoginFlowHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/x/passport-login/web/qrcode/generate", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"code":0,"data":{"qrcode_key":"KEY","url":"https://qr.example/1"}}`)
	})
	mux.HandleFunc("/x/passport-login/web/qrcode/poll", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "SESSDATA=abc; Path=/; Domain=.bilibili.com")
		w.Header().Add("Set-Cookie", "DedeUserID=42; Path=/")
		_, _ = fmt.Fprint(w, `{"data":{"code":0}}`)
	})
	b := &logintest.Browser{}
	env, srv := logintest.Env(mux, b)
	t.Cleanup(srv.Close)

	sess := &login.Session{}
	qr, qrText, err := biliHTTPStart(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if qr != "" || qrText != "https://qr.example/1" {
		t.Fatalf("start = %q / %q", qr, qrText)
	}
	done, cookies, user, err := biliHTTPPoll(context.Background(), env, sess)
	if err != nil {
		t.Fatal(err)
	}
	if !done || user != "42" || cookies["SESSDATA"] != "abc" {
		t.Fatalf("poll = done=%v user=%q cookies=%v", done, user, cookies)
	}
	injected := false
	for _, c := range b.Injected {
		if c.Name == "SESSDATA" && c.Domain == ".bilibili.com" {
			injected = true
		}
	}
	if !injected {
		t.Fatalf("cookie 未回灌载具: %+v", b.Injected)
	}
}

func TestLoginFlowExpired(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/x/passport-login/web/qrcode/generate", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"code":0,"data":{"qrcode_key":"KEY","url":"https://qr.example/1"}}`)
	})
	mux.HandleFunc("/x/passport-login/web/qrcode/poll", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":{"code":86038}}`)
	})
	b := &logintest.Browser{}
	env, srv := logintest.Env(mux, b)
	t.Cleanup(srv.Close)
	sess := &login.Session{}
	if _, _, err := biliHTTPStart(context.Background(), env, sess); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := biliHTTPPoll(context.Background(), env, sess)
	if err != login.ErrExpired {
		t.Fatalf("expired err = %v", err)
	}
}
