// 微博登录判定回归:匿名访客也有 SUB+SUBP,只有完整登录标记才算登录。

package weibo

import "testing"

func TestLoginFlowIgnoresAnonymousVisitor(t *testing.T) {
	flow := LoginFlow()
	cases := []struct {
		name    string
		cookies map[string]string
		want    bool
	}{
		{"无 cookie", map[string]string{}, false},
		{"匿名 SUB+SUBP", map[string]string{"SUB": "s1", "SUBP": "12345"}, false},
		{"SSOLoginState", map[string]string{"SUB": "s1", "SSOLoginState": "1"}, true},
		{"ALF", map[string]string{"SUB": "s1", "ALF": "token"}, true},
		{"SUL", map[string]string{"SUB": "s1", "SUL": "1"}, true},
		{"MLOGIN=1", map[string]string{"SUB": "s1", "MLOGIN": "1"}, true},
		{"MLOGIN=0", map[string]string{"SUB": "s1", "MLOGIN": "0"}, false},
		{"登录标记但无 SUB", map[string]string{"SUBP": "12345", "ALF": "token"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flow.LoggedIn(tc.cookies); got != tc.want {
				t.Fatalf("LoggedIn(%v) = %v, want %v", tc.cookies, got, tc.want)
			}
		})
	}
}

func TestLoginFlowUserDecodesSUBP(t *testing.T) {
	flow := LoginFlow()
	if got := flow.User(map[string]string{"SUBP": "12%2634"}); got != "12&34" {
		t.Fatalf("User = %q, want URL 解码后的 SUBP", got)
	}
	if got := flow.User(map[string]string{}); got != "" {
		t.Fatalf("缺少 SUBP 时 User = %q", got)
	}
}
