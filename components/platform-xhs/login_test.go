// 小红书扫码登录判据回归:user_id 相对基线变化即成功;同账号重登
// (user_id 不变)必须靠登录页页面态翻转确认。

package xhs

import (
	"context"
	"testing"

	"mygo-host/platform/login"
	"mygo-host/platform/login/logintest"
)

func TestLoginFlowMetadata(t *testing.T) {
	flow := LoginFlow()
	if flow.ID != "xhs" || flow.TrustCookies {
		t.Fatalf("flow = %+v, want xhs 且不信残留 cookie", flow)
	}
	if flow.BaselineCookie != "user_id" || flow.Open == nil || flow.PollCheck == nil {
		t.Fatalf("flow 登录配方不完整: %+v", flow)
	}
}

func TestXhsPollCheckBaselineChangeWinsOverPageState(t *testing.T) {
	b := &logintest.Browser{}
	done, user := xhsPollCheck(context.Background(), &login.Env{Browser: b},
		&login.Session{BaselineUserID: "u1"}, map[string]string{"user_id": "u2"})
	if !done || user != "u2" {
		t.Fatalf("poll = %v/%q, want true/u2", done, user)
	}
	if b.DoAsCalls != 0 {
		t.Fatalf("user_id 已变化仍读页面态: DoAs=%d", b.DoAsCalls)
	}
}

func TestXhsPollCheckSameAccountUsesPageState(t *testing.T) {
	cases := []struct {
		name       string
		pageLogged any
		domLogged  bool
		want       bool
	}{
		{"同账号重登页面态翻转", true, false, true},
		{"页面态 false", false, false, false},
		{"SSR 读不到时 DOM 兜底", nil, true, true},
		{"无登录证据", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, page := newFakeBrowser()
			page.pageLogged, page.domLogged = tc.pageLogged, tc.domLogged
			b := &logintest.Browser{Page: page}
			done, user := xhsPollCheck(context.Background(), &login.Env{Browser: b},
				&login.Session{BaselineUserID: "u1"}, map[string]string{"user_id": "u1"})
			if done != tc.want || user != "u1" {
				t.Fatalf("poll = %v/%q, want %v/u1(calls=%v)", done, user, tc.want, fake.calls)
			}
			if b.DoAsCalls != 1 {
				t.Fatalf("页面态判定 DoAs=%d, want 1", b.DoAsCalls)
			}
		})
	}
}

func TestXhsPollCheckIgnoresGuestWebSession(t *testing.T) {
	b := &logintest.Browser{}
	done, user := xhsPollCheck(context.Background(), &login.Env{Browser: b},
		&login.Session{}, map[string]string{"web_session": "guest"})
	if done || user != "" {
		t.Fatalf("游客 web_session 被当成登录: %v/%q(DoAs=%d)", done, user, b.DoAsCalls)
	}
}
