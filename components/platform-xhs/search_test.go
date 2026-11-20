// 关键词搜用户链路测试:调用顺序、登录判定链、风控、SSR 投影解析与错误语义。

package xhs

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"mygo-host/platform"
)

const twoUsersState = `[
  {"userId":"u1","nickname":"美食博主","avatar":"https://img/x.jpg","redId":"xhs001","fans":12345},
  {"userId":"u2","nickname":"旅行者","avatar":"","redId":"","fans":0}
]`

const origin = "https://www.xiaohongshu.com"

func TestSearchUsersFlow(t *testing.T) {
	fake, page := newFakeBrowser()
	page.pageLogged = true
	page.state = twoUsersState

	items, err := searchUsers(context.Background(), testContext(fake), "美食 博主")
	if err != nil {
		t.Fatal(err)
	}
	assertMappedItems(t, items)

	assertCalls(t, fake.calls, []string{
		"Do:" + origin,
		"CaptureNavigate",
		"Risk",
		"LoginPage",
		"PageLoggedIn",
		"Capture",
		"Click",
		"State",
		"CloseTab:" + origin,
	})
	wantURL := origin + "/search_result?keyword=%E7%BE%8E%E9%A3%9F%20%E5%8D%9A%E4%B8%BB" +
		"&source=web_search_result_notes&type=51"
	if page.navCaptureURL != wantURL {
		t.Errorf("navURL = %q, want %q", page.navCaptureURL, wantURL)
	}
	assertCaptureParams(t, page)
	if !page.clicked {
		t.Error("未点击「用户」Tab")
	}
}

// assertCaptureParams 两段捕获窗口必须对齐规格(2500/20000 与 2000/8000),
// 且两段都用 isXhsSearchAPI 过滤。
func assertCaptureParams(t *testing.T, page *fakePage) {
	t.Helper()
	if page.navIdle != 2500*time.Millisecond || page.navTimeout != 20*time.Second {
		t.Errorf("导航捕获 idle/timeout = %v/%v", page.navIdle, page.navTimeout)
	}
	if page.clickIdle != 2*time.Second || page.clickTimeout != 8*time.Second {
		t.Errorf("点击捕获 idle/timeout = %v/%v", page.clickIdle, page.clickTimeout)
	}
	api, err := url.Parse(origin + "/api/sns/web/v1/search/usersearch")
	if err != nil {
		t.Fatal(err)
	}
	pageURL, _ := url.Parse(origin + "/search_result")
	for i, match := range []func(*url.URL) bool{page.navMatch, page.clickMatch} {
		if match == nil || !match(api) || match(pageURL) {
			t.Errorf("match[%d] 不是 isXhsSearchAPI", i)
		}
	}
}

// TestSearchUsersCapturedResults 捕获到 usersearch 结果时直接返回,不读 SSR。
func TestSearchUsersCapturedResults(t *testing.T) {
	fake, page := newFakeBrowser()
	page.pageLogged = true
	page.state = twoUsersState
	page.navResponses = []platform.CapturedResponse{{
		URL:  origin + "/api/sns/web/v1/search/usersearch",
		Body: `{"data":{"users":[{"id":"u9","nickname":"捕获用户","red_id":"r9","fans":"1.2万"}]}}`,
	}}

	items, err := searchUsers(context.Background(), testContext(fake), "美食")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "捕获用户" ||
		items[0].Summary == nil || *items[0].Summary != "小红书号 r9 · 1.2万 粉丝" {
		t.Fatalf("items = %+v", items)
	}
	assertNoCall(t, fake.calls, "State")
}

// TestSearchUsersEmptyResultPackSkipsSSR 本次窗口有结果包(即使是空数组)时,
// 不回落 SSR(state 里的推荐用户不得当成关键词结果)。
func TestSearchUsersEmptyResultPackSkipsSSR(t *testing.T) {
	fake, page := newFakeBrowser()
	page.pageLogged = true
	page.state = twoUsersState
	page.clickResponses = []platform.CapturedResponse{{
		URL:  origin + "/api/sns/web/v1/search/usersearch?keyword=kw",
		Body: `{"data":{"users":[]}}`,
	}}

	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	want := "没有搜到与「kw」相关的用户：换个关键词，或确认搜索页能正常打开（可能需要登录）"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	assertNoCall(t, fake.calls, "State")
}

func assertMappedItems(t *testing.T, items []platform.Item) {
	t.Helper()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	first := items[0]
	if first.ID != "xu-u1" || first.SourceID != sourceID || first.Kind != platform.KindVideo {
		t.Errorf("items[0] id/source/kind = %s/%s/%s", first.ID, first.SourceID, first.Kind)
	}
	if first.Title != "美食博主" || first.FetchedAt != testNow {
		t.Errorf("items[0] title/fetchedAt = %s/%d", first.Title, first.FetchedAt)
	}
	if first.URL == nil || *first.URL != origin+"/user/profile/u1" {
		t.Errorf("items[0] url = %v", first.URL)
	}
	if first.Thumbnail == nil || *first.Thumbnail != "https://img/x.jpg" {
		t.Errorf("items[0] thumbnail = %v", first.Thumbnail)
	}
	if first.Summary == nil || *first.Summary != "小红书号 xhs001 · 1.2万 粉丝" {
		t.Errorf("items[0] summary = %v", first.Summary)
	}
	if items[1].ID != "xu-u2" || items[1].Thumbnail != nil || items[1].Summary != nil {
		t.Errorf("items[1] = %+v", items[1])
	}
}

func TestSearchUsersRequiresBrowser(t *testing.T) {
	_, err := searchUsers(context.Background(), testContext(nil), "kw")
	if !errors.Is(err, platform.ErrBrowserUnavailable) {
		t.Fatalf("err = %v, want ErrBrowserUnavailable", err)
	}
}

func TestSearchUsersRiskControl(t *testing.T) {
	fake, page := newFakeBrowser()
	page.risk = true

	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	if !errors.Is(err, platform.ErrBrowserRiskControl) {
		t.Fatalf("err = %v, want ErrBrowserRiskControl", err)
	}
	if !strings.Contains(err.Error(), ".fe-verify-box") {
		t.Errorf("err = %v, want 含风控框文案", err)
	}
	assertCalls(t, fake.calls, []string{
		"Do:" + origin, "CaptureNavigate", "Risk", "CloseTab:" + origin,
	})
	if page.clicked {
		t.Error("风控后不应继续点击/读 state")
	}
}

func TestSearchUsersLoginRequired(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(*fakePage)
		forbidden []string
	}{
		{"登录页", func(p *fakePage) { p.loginPage = true }, []string{"PageLoggedIn", "DOM", "Cookies"}},
		{"SSR loggedIn=false", func(p *fakePage) { p.pageLogged = false }, []string{"DOM", "Cookies"}},
		{"只有 web_session", func(p *fakePage) {
			p.cookies = map[string]string{"web_session": "guest"}
		}, nil},
		{"cookie 读取失败", func(p *fakePage) { p.cookieErr = errors.New("cdp down") }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, page := newFakeBrowser()
			tc.setup(page)
			_, err := searchUsers(context.Background(), testContext(fake), "kw")
			if !errors.Is(err, platform.ErrBrowserLoginRequired) {
				t.Fatalf("err = %v, want ErrBrowserLoginRequired", err)
			}
			if !strings.Contains(err.Error(), "登录态与密钥") {
				t.Errorf("err = %v, want 指向扫码登录页", err)
			}
			if fake.closed != 1 {
				t.Errorf("closed = %d, want 1", fake.closed)
			}
			assertNoCall(t, fake.calls, append(tc.forbidden, "State", "Click", "Capture")...)
			assertOrder(t, fake.calls, "Do:"+origin, "CaptureNavigate", "Risk", "CloseTab:"+origin)
		})
	}
}

func TestSearchUsersLoginFallbackDOM(t *testing.T) {
	fake, page := newFakeBrowser()
	page.domLogged = true
	page.state = twoUsersState

	if _, err := searchUsers(context.Background(), testContext(fake), "kw"); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, fake.calls, []string{
		"Do:" + origin, "CaptureNavigate", "Risk", "LoginPage", "PageLoggedIn",
		"DOM", "Capture", "Click", "State", "CloseTab:" + origin,
	})
}

func TestSearchUsersLoginFallbackCookie(t *testing.T) {
	fake, page := newFakeBrowser()
	page.cookies = map[string]string{"user_id": "abc"}
	page.state = twoUsersState

	if _, err := searchUsers(context.Background(), testContext(fake), "kw"); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, fake.calls, []string{
		"Do:" + origin, "CaptureNavigate", "Risk", "LoginPage", "PageLoggedIn",
		"DOM", "Cookies", "Capture", "Click", "State", "CloseTab:" + origin,
	})
}

func TestSearchUsersLoginBitReadErrorFallsThrough(t *testing.T) {
	fake, page := newFakeBrowser()
	page.evalErrs = map[string]error{"PageLoggedIn": errors.New("execution context destroyed")}
	page.domLogged = true
	page.state = twoUsersState

	if _, err := searchUsers(context.Background(), testContext(fake), "kw"); err != nil {
		t.Fatal(err)
	}
	assertOrder(t, fake.calls, "PageLoggedIn", "DOM", "Capture", "State", "CloseTab:"+origin)
}

func TestSearchUsersNoResults(t *testing.T) {
	cases := []struct {
		name  string
		state any
	}{
		{"空数组", "[]"},
		{"空字符串", ""},
		{"nil", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, page := newFakeBrowser()
			page.pageLogged = true
			page.state = tc.state
			_, err := searchUsers(context.Background(), testContext(fake), "kw")
			want := "没有搜到与「kw」相关的用户：换个关键词，或确认搜索页能正常打开（可能需要登录）"
			if err == nil || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
			if fake.closed != 1 {
				t.Errorf("closed = %d, want 1", fake.closed)
			}
		})
	}
}

func TestSearchUsersStateShapeErrors(t *testing.T) {
	cases := []struct {
		name    string
		state   any
		wantSub string
	}{
		{"返回非字符串", float64(1), "返回类型异常"},
		{"JSON 非法", "{not json", "投影解析失败"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, page := newFakeBrowser()
			page.pageLogged = true
			page.state = tc.state
			_, err := searchUsers(context.Background(), testContext(fake), "kw")
			if err == nil || !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("err = %v, want 含 %q", err, tc.wantSub)
			}
			if fake.closed != 1 {
				t.Errorf("closed = %d, want 1", fake.closed)
			}
		})
	}
}

func TestSearchUsersClosesTabOnDoError(t *testing.T) {
	fake, _ := newFakeBrowser()
	fake.doErr = errors.New("载具启动失败")

	_, err := searchUsers(context.Background(), testContext(fake), "kw")
	if err == nil || err.Error() != "载具启动失败" {
		t.Fatalf("err = %v", err)
	}
	assertCalls(t, fake.calls, []string{"Do:" + origin, "CloseTab:" + origin})
}

func TestSearchUsersIgnoresCloseAndClickErrors(t *testing.T) {
	fake, page := newFakeBrowser()
	fake.closeErr = errors.New("close failed")
	page.pageLogged = true
	page.state = twoUsersState
	page.clickErr = errors.New("execution context destroyed")

	items, err := searchUsers(context.Background(), testContext(fake), "kw")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(items) != 2 || fake.closed != 1 {
		t.Errorf("items/closed = %d/%d", len(items), fake.closed)
	}
	if !page.clicked {
		t.Error("点击「用户」Tab 未被尝试")
	}
}
