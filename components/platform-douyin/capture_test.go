// 捕获解析测试:搜索接口判定、两种响应形状、secUid 去重与字段归一
// (user_info 回落、头像 ?? 语义、nickname 末 8 位回退)。

package douyin

import (
	"context"
	"net/url"
	"testing"

	"mygo-host/platform"
)

// TestSearchClosesTabWhenContextCanceled 搜索超时(父 ctx 已取消)也必须关闭搜索页,
// 否则 target 与 renderer 会常驻到载具重启。
func TestSearchClosesTabWhenContextCanceled(t *testing.T) {
	browser := &fakeBrowser{
		cookies:  map[string]string{"sessionid": "s"},
		captured: []platform.CapturedResponse{{Body: string(fixture(t, "search_users_top.json"))}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := searchByKeyword(ctx, &platform.ServiceContext{
		Browser: browser,
		Now:     func() int64 { return testNow },
	}, "kw"); err != nil {
		t.Fatal(err)
	}
	if browser.closeCalls != 1 || browser.closeCtxErr != nil {
		t.Fatalf("closeCalls/closeCtxErr = %d/%v, want 1/nil", browser.closeCalls, browser.closeCtxErr)
	}
}

func TestIsSearchAPI(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"general", "https://www.douyin.com/aweme/v1/web/general/search/single/?keyword=x", true},
		{"discover", "https://www.douyin.com/aweme/v1/web/discover/search/single/", true},
		{"user", "https://www.douyin.com/aweme/v1/web/search/user/?keyword=x", true},
		{"apex", "https://douyin.com/aweme/v1/web/search/user/", true},
		{"profile", "https://www.douyin.com/aweme/v1/web/user/profile/other/", false},
		{"伪域名", "https://evil-douyin.com/aweme/v1/web/search/user/", false},
		{"后缀伪装", "https://douyin.com.evil.com/aweme/v1/web/search/user/", false},
		{"搜索页本身", "https://www.douyin.com/search/abc?type=user", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := url.Parse(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := isSearchAPI(parsed); got != tc.want {
				t.Errorf("isSearchAPI(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseCapturedUsersShapes(t *testing.T) {
	responses := []platform.CapturedResponse{
		{Body: string(fixture(t, "search_users_top.json"))},
		{Body: string(fixture(t, "search_users_wrapped.json"))},
	}
	hits := parseCapturedUsers(responses)
	want := []userHit{
		{secUID: "MS4wLjABAAAA1234567890abcdef", nickname: "测试用户", avatar: "https://p3.douyinpic.com/thumb.jpg", uniqueID: "test123", followers: 12345, hasFollowers: true},
		{secUID: "MS4wLjABAAAA0987654321fedcba", nickname: "21fedcba"},
		{secUID: "MS4wLjABAAAAwrapped01", nickname: "包裹用户", avatar: "https://p3.douyinpic.com/wrap.jpg", uniqueID: "wrap"},
		{secUID: "MS4wLjABAAAAnullinfo", nickname: "空信息回落"},
	}
	if len(hits) != len(want) {
		t.Fatalf("hits = %d, want %d: %+v", len(hits), len(want), hits)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Errorf("hits[%d] = %+v, want %+v", i, hits[i], want[i])
		}
	}
}

func TestParseCapturedUsersSkipsBadJSON(t *testing.T) {
	responses := []platform.CapturedResponse{
		{Body: "not json"},
		{Body: string(fixture(t, "search_users_top.json"))},
	}
	if hits := parseCapturedUsers(responses); len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestUserHitFromRequiresSecUID(t *testing.T) {
	if _, ok := userHitFrom(map[string]any{"nickname": "x"}); ok {
		t.Fatal("缺 sec_uid 应跳过")
	}
}
