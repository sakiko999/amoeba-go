// 浏览器采集链路测试:调用顺序、搜索页 URL、匹配器、错误与空结果语义。

package weibo

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestSearchUsersFlow(t *testing.T) {
	keyword := "半佛仙人"
	fake := &fakeBrowser{responses: []platform.CapturedResponse{{
		URL:  capturedURL(userContainer(keyword)),
		Body: fixture(t, "user_search.json"),
	}}}
	items, err := searchUsers(context.Background(), testContext(fake), keyword)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ID != "wbu-1234567890" || items[0].Title != "半佛仙人" {
		t.Errorf("items[0] = %s/%s", items[0].ID, items[0].Title)
	}
	assertCalls(t, fake.calls, []string{
		"Do:https://m.weibo.cn",
		"CaptureNavigate:https://m.weibo.cn/search?containerid=100103type%3D3%26q%3D%E5%8D%8A%E4%BD%9B%E4%BB%99%E4%BA%BA",
		"CloseTab:https://m.weibo.cn",
	})
	if fake.origin != "https://m.weibo.cn" {
		t.Errorf("origin = %q", fake.origin)
	}
	if fake.idle != 2*time.Second || fake.timeout != 25*time.Second {
		t.Errorf("idle/timeout = %v/%v", fake.idle, fake.timeout)
	}
}

func TestSearchPageURL(t *testing.T) {
	cases := []struct{ keyword, want string }{
		{"半佛仙人", "https://m.weibo.cn/search?containerid=100103type%3D3%26q%3D%E5%8D%8A%E4%BD%9B%E4%BB%99%E4%BA%BA"},
		{"a b", "https://m.weibo.cn/search?containerid=100103type%3D3%26q%3Da+b"},
		{"a&b", "https://m.weibo.cn/search?containerid=100103type%3D3%26q%3Da%26b"},
	}
	for _, tc := range cases {
		t.Run(tc.keyword, func(t *testing.T) {
			if got := searchPageURL(tc.keyword); got != tc.want {
				t.Errorf("searchPageURL(%q) = %q, want %q", tc.keyword, got, tc.want)
			}
		})
	}
}

func TestIsWeiboUserSearchAPI(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://m.weibo.cn/api/container/getIndex?containerid=100103type%3D3%26q%3Dx", true},
		{"https://a.m.weibo.cn/api/container/getIndex?containerid=100103type%3D3%26q%3Dx&display=0", true},
		{"https://m.weibo.cn/api/container/getIndex?containerid=100103type%3D1%26q%3Dx", false},
		{"https://m.weibo.cn/api/container/getIndex", false},
		{"https://m.weibo.cn/api/container/getIndex/", false},
		{"https://weibo.com/api/container/getIndex?containerid=100103type%3D3%26q%3Dx", false},
		{"https://weibo.cn/api/container/getIndex?containerid=100103type%3D3%26q%3Dx", false},
		{"https://m.weibo.cn.evil.com/api/container/getIndex?containerid=100103type%3D3%26q%3Dx", false},
		{"https://evilm.weibo.cn/api/container/getIndex?containerid=100103type%3D3%26q%3Dx", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			parsed, err := url.Parse(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			if got := isWeiboUserSearchAPI(parsed); got != tc.want {
				t.Errorf("match = %v, want %v", got, tc.want)
			}
		})
	}
	if isWeiboUserSearchAPI(nil) {
		t.Error("nil URL 应为 false")
	}
}

func TestSearchUsersClosesTabOnDoError(t *testing.T) {
	fake := &fakeBrowser{doErr: errors.New("载具启动失败")}
	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	if err == nil || err.Error() != "载具启动失败" {
		t.Fatalf("err = %v", err)
	}
	assertCalls(t, fake.calls, []string{"Do:https://m.weibo.cn", "CloseTab:https://m.weibo.cn"})
}

func TestSearchUsersClosesTabOnCaptureError(t *testing.T) {
	fake := &fakeBrowser{captureErr: errors.New("捕获失败")}
	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	if err == nil || err.Error() != "捕获失败" {
		t.Fatalf("err = %v", err)
	}
	if fake.closed != 1 {
		t.Errorf("closed = %d, want 1", fake.closed)
	}
}

func TestSearchUsersRiskControl(t *testing.T) {
	fake := &fakeBrowser{responses: []platform.CapturedResponse{{
		URL:  capturedURL(userContainer("kw")),
		Body: `{"ok":-100,"msg":"请求过于频繁"}`,
	}}}
	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	if !errors.Is(err, platform.ErrBrowserRiskControl) {
		t.Fatalf("err = %v, want ErrBrowserRiskControl", err)
	}
	if !strings.Contains(err.Error(), "请求过于频繁") {
		t.Errorf("err = %v, want 含限流文案", err)
	}
}

func TestSearchUsersNoResults(t *testing.T) {
	cases := []struct {
		name      string
		responses []platform.CapturedResponse
	}{
		{"无捕获响应", nil},
		{"ok=1 空列表", []platform.CapturedResponse{{
			URL: capturedURL(userContainer("kw")), Body: `{"ok":1,"data":{"cards":[]}}`,
		}}},
		{"ok=1 但用户卡全被过滤", []platform.CapturedResponse{{
			URL:  capturedURL(userContainer("kw")),
			Body: `{"ok":1,"data":{"cards":[{"user":{"id":1,"screen_name":"  "}}]}}`,
		}}},
		{"JSON 解析失败", []platform.CapturedResponse{{
			URL: capturedURL(userContainer("kw")), Body: `{not json`,
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeBrowser{responses: tc.responses}
			_, err := searchUsers(context.Background(), testContext(fake), "kw")
			if err == nil || err.Error() != noResultsMessage {
				t.Fatalf("err = %v, want %q", err, noResultsMessage)
			}
			if fake.closed != 1 {
				t.Errorf("closed = %d, want 1", fake.closed)
			}
		})
	}
}
