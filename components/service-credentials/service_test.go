// 服务面单测：登录态以 cookie 镜像为准、登录成功镜像进 credentials.json、
// 退出登录清域 cookie、刷新复用 pin、旧轮询不误关心会话、密钥脱敏。
// 平台私有配方不在这里测，见各 platform-* 包的 login*_test.go。

package credentials

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mygo-host/components/platform-bangumi"
	"mygo-host/components/platform-tmdb"
	"mygo-host/platform"
	"mygo-host/platform/login"
	"mygo-host/platform/login/logintest"
)

func newTestService(t *testing.T, b login.Browser) (*Service, *platform.Credentials) {
	t.Helper()
	creds := platform.NewCredentials(filepath.Join(t.TempDir(), "credentials.json"))
	svc, _, err := NewService(creds, b, nil, t.Logf, []login.Flow{
		fakeFlow("weibo", "微博", []string{"weibo.com", "weibo.cn"}, "SUB"),
		fakeFlow("xhs", "小红书", []string{"xiaohongshu.com"}, "user_id"),
		fakeFlow("douyin", "抖音", []string{"douyin.com"}, "sessionid"),
		fakeFlow("douban", "豆瓣", []string{"douban.com"}, "dbcl2"),
		fakeFlow("bili", "B站", []string{"bilibili.com"}, "SESSDATA"),
	}, []platform.KeySpec{
		{Name: tmdb.KeyName, Label: tmdb.KeyLabel},
		{Name: bangumi.KeyName, Label: bangumi.KeyLabel},
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, creds
}

func fakeFlow(id, label string, hosts []string, loginCookie string) login.Flow {
	return login.Flow{
		ID: id, Label: label, Hosts: hosts, Origin: "https://example.com",
		LoginCookie: loginCookie, Timeout: time.Minute, TrustCookies: true,
	}
}

// browserFlow 模拟需要登录页的浏览器平台：Open 固定返回 already。
func browserFlow(id, label string, loginCookie string, already bool) login.Flow {
	f := fakeFlow(id, label, []string{"example.com"}, loginCookie)
	f.TrustCookies = false
	f.BaselineCookie = loginCookie
	f.Open = func(context.Context, *login.Env, platform.BrowserPage) (string, bool, error) {
		return "", already, nil
	}
	return f
}

func cookie(name, value, domain string) platform.BrowserCookie {
	return platform.BrowserCookie{Name: name, Value: value, Domain: domain, Path: "/"}
}

func TestStatus(t *testing.T) {
	svc, creds := newTestService(t, &logintest.Browser{})
	if err := creds.Set("weibo", "SUB=s1; SUBP=12345"); err != nil {
		t.Fatal(err)
	}
	if err := creds.Set("xhs", "user_id=u1"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]PlatformStatus{}
	for _, p := range res.Platforms {
		byID[p.ID] = p
	}
	if !byID["weibo"].LoggedIn || byID["weibo"].User != "" || !byID["weibo"].Stored {
		t.Fatalf("weibo 状态 = %+v", byID["weibo"])
	}
	if !byID["xhs"].LoggedIn || !byID["xhs"].Stored {
		t.Fatalf("xhs 状态 = %+v", byID["xhs"])
	}
	if byID["douyin"].LoggedIn || byID["douban"].LoggedIn {
		t.Fatalf("douyin/douban 不应登录: %+v", byID)
	}
}

func TestLogoutClearsDomainAndMirror(t *testing.T) {
	b := &logintest.Browser{}
	svc, creds := newTestService(t, b)
	if err := creds.Set("weibo", "SUB=s1"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Logout(context.Background(), PlatformParams{Platform: "weibo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Cleared) != 1 || len(b.Cleared[0]) != 2 {
		t.Fatalf("Logout = %+v cleared=%v", res, b.Cleared)
	}
	if got := strings.Join(b.Cleared[0], ","); got != "weibo.com,weibo.cn" {
		t.Fatalf("清除域名 = %s", got)
	}
	if _, ok := creds.Lookup("weibo"); ok {
		t.Fatal("退出登录后镜像未清除")
	}
}

func TestLoginStartAlreadyTrusted(t *testing.T) {
	b := &logintest.Browser{Cookies: []platform.BrowserCookie{
		cookie("SUB", "s1", ".weibo.com"), cookie("SUBP", "12345", ".weibo.com"),
	}}
	svc, creds := newTestService(t, b)
	if err := creds.Set("weibo", "SUB=s1"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.LoginStart(context.Background(), PlatformParams{Platform: "weibo"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "already" || b.DoAsCalls != 0 {
		t.Fatalf("LoginStart = %+v doAs=%d", res, b.DoAsCalls)
	}
	if got, _ := creds.Lookup("weibo"); !strings.Contains(got, "SUB=s1") {
		t.Fatalf("cookie 未镜像: %q", got)
	}
}

// 刷新会再次 LoginStart;已有登录 pin 时必须复用 CookiesAs,不能重新
// Acquire 同一 profile 把请求卡到旧会话释放。
func TestLoginStartRefreshReusesPinnedCookies(t *testing.T) {
	b := &logintest.Browser{Cookies: []platform.BrowserCookie{cookie("user_id", "u1", ".example.com")}}
	svc, _ := newTestService(t, b)
	svc.flows["xhs"] = browserFlow("xhs", "小红书", "user_id", true)
	svc.setSession(&login.Session{Platform: "xhs", UsedBrowser: true, Deadline: time.Now().Add(time.Minute)})
	_, _ = svc.LoginStart(context.Background(), PlatformParams{Platform: "xhs"})
	if b.CookieAs == 0 {
		t.Fatal("刷新未走 CookiesAs,会重新 Acquire 被 pin 住的 profile")
	}
}

// 浏览器路径返回 already 时也要镜像 cookie。
func TestLoginStartBrowserAlready(t *testing.T) {
	b := &logintest.Browser{
		Cookies: []platform.BrowserCookie{cookie("user_id", "u1", ".example.com")},
		Page:    &logintest.Page{},
	}
	svc, creds := newTestService(t, b)
	svc.flows["xhs"] = browserFlow("xhs", "小红书", "user_id", true)
	res, err := svc.LoginStart(context.Background(), PlatformParams{Platform: "xhs"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "already" || b.DoAsCalls != 1 {
		t.Fatalf("LoginStart = %+v doAs=%d", res, b.DoAsCalls)
	}
	if got, _ := creds.Lookup("xhs"); !strings.Contains(got, "user_id=u1") {
		t.Fatalf("cookie 未镜像: %q", got)
	}
}

// 旧轮询迟到时不能删除/关闭刷新后创建的新会话。
func TestStalePollDoesNotDropNewSession(t *testing.T) {
	svc, _ := newTestService(t, &logintest.Browser{})
	old := &login.Session{Platform: "xhs", Deadline: time.Now().Add(time.Minute)}
	cur := &login.Session{Platform: "xhs", Deadline: time.Now().Add(time.Minute)}
	svc.setSession(cur)
	if svc.dropSessionIf("xhs", old) {
		t.Fatal("旧轮询不应删除新会话")
	}
	if svc.session("xhs") != cur {
		t.Fatal("新会话被旧轮询删除")
	}
}

func TestLoginPollMirrorsCookies(t *testing.T) {
	b := &logintest.Browser{Cookies: []platform.BrowserCookie{cookie("user_id", "u2", ".example.com")}}
	svc, creds := newTestService(t, b)
	svc.flows["xhs"] = browserFlow("xhs", "小红书", "user_id", false)
	svc.setSession(&login.Session{Platform: "xhs", BaselineUserID: "u1", Deadline: time.Now().Add(time.Minute)})
	res, err := svc.LoginPoll(context.Background(), PlatformParams{Platform: "xhs"})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "success" {
		t.Fatalf("LoginPoll = %+v", res)
	}
	if got, _ := creds.Lookup("xhs"); !strings.Contains(got, "user_id=u2") {
		t.Fatalf("cookie 未镜像: %q", got)
	}
}

func TestKeysMasking(t *testing.T) {
	svc, _ := newTestService(t, &logintest.Browser{})
	ctx := context.Background()
	if err := svc.SetKey(ctx, SetKeyParams{Name: "tmdb", Value: "  abcd1234  "}); err != nil {
		t.Fatal(err)
	}
	keys, err := svc.Keys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].Name != "bangumi" || keys[1].Name != "tmdb" {
		t.Fatalf("keys = %+v", keys)
	}
	if !keys[1].Set || keys[1].Masked != "••••1234" {
		t.Fatalf("tmdb 脱敏 = %+v", keys[1])
	}
	if err := svc.SetKey(ctx, SetKeyParams{Name: "tmdb", Value: ""}); err != nil {
		t.Fatal(err)
	}
	keys, _ = svc.Keys(ctx)
	if keys[1].Set {
		t.Fatalf("清除后仍标记已设置: %+v", keys[1])
	}
	if err := svc.SetKey(ctx, SetKeyParams{Name: "nope", Value: "x"}); err == nil {
		t.Fatal("未知密钥名应报错")
	}
}
