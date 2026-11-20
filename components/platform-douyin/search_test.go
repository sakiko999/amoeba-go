// 搜索测试:查号两条路径(主页链接/sec_uid 免网络、抖音号走匿名资料接口),
// 关键词路径的登录门禁两态与捕获解析(fake Browser),以及浏览器不可用/无结果。

package douyin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"mygo-host/platform"
)

// withUserInfoServer 覆写资料接口地址;返回请求计数指针。
func withUserInfoServer(t *testing.T, handler http.HandlerFunc) *int {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		handler(w, r)
	}))
	old := userInfoAPI
	userInfoAPI = srv.URL + "/web/api/v2/user/info/"
	t.Cleanup(func() {
		userInfoAPI = old
		srv.Close()
	})
	return &calls
}

func TestLookupByAccountProfileLink(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	calls := withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("主页链接不应请求资料接口")
	})
	hit, err := lookupByAccount(context.Background(), testContext(),
		"https://www.douyin.com/user/MS4wLjABAAAAabc_123-xyz?from=share")
	if err != nil {
		t.Fatal(err)
	}
	if hit == nil || hit.secUID != "MS4wLjABAAAAabc_123-xyz" {
		t.Fatalf("hit = %+v", hit)
	}
	if hit.nickname != lastN(hit.secUID, 8) || *calls != 0 {
		t.Errorf("nickname = %q, calls = %d", hit.nickname, *calls)
	}
}

func TestLookupByAccountBareSecUID(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("裸 sec_uid 不应请求资料接口")
	})
	hit, err := lookupByAccount(context.Background(), testContext(), "MS4wLjABAAAAsecret")
	if err != nil || hit == nil || hit.secUID != "MS4wLjABAAAAsecret" {
		t.Fatalf("hit = %+v, err = %v", hit, err)
	}
	if hit.nickname != lastN("MS4wLjABAAAAsecret", 8) {
		t.Errorf("nickname = %q", hit.nickname)
	}
}

func TestLookupByAccountWhitespace(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	calls := withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("带空白的输入应视为关键词")
	})
	hit, err := lookupByAccount(context.Background(), testContext(), "星 海")
	if hit != nil || err != nil {
		t.Fatalf("hit = %+v, err = %v", hit, err)
	}
	if *calls != 0 {
		t.Errorf("calls = %d", *calls)
	}
}

func TestLookupByAccountAPIAndCache(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	var query url.Values
	var header http.Header
	calls := withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		query, header = r.URL.Query(), r.Header.Clone()
		w.Write(fixture(t, "user_info.json"))
	})
	sc := testContext()
	hit, err := lookupByAccount(context.Background(), sc, "抖音号：pvcyinse")
	if err != nil {
		t.Fatal(err)
	}
	assertUserInfoHit(t, hit)
	assertUserInfoRequest(t, query, header)
	if _, err := lookupByAccount(context.Background(), sc, "抖音号：pvcyinse"); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Errorf("同抖音号第二次仍请求(缓存失效), calls = %d", *calls)
	}
}

func assertUserInfoHit(t *testing.T, hit *userHit) {
	t.Helper()
	if hit == nil || hit.secUID != "MS4wLjABAAAApvcyinse" || hit.nickname != "测试主播" {
		t.Fatalf("hit = %+v", hit)
	}
	if hit.uniqueID != "pvcyinse" || hit.avatar != "https://p3.douyinpic.com/pvcyinse.jpg" {
		t.Errorf("uniqueID/avatar = %q/%q", hit.uniqueID, hit.avatar)
	}
}

func assertUserInfoRequest(t *testing.T, query url.Values, header http.Header) {
	t.Helper()
	if query.Get("unique_id") != "pvcyinse" {
		t.Errorf("query = %v", query)
	}
	if header.Get("user-agent") != desktopChromeUA || header.Get("referer") != "https://www.douyin.com/" {
		t.Errorf("headers = %v", header)
	}
}

func TestLookupByAccountNotFound(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status_code":1,"status_msg":"用户不存在"}`)
	})
	hit, err := lookupByAccount(context.Background(), testContext(), "nouser")
	if hit != nil || err != nil {
		t.Fatalf("查无此人应回退关键词: hit = %+v, err = %v", hit, err)
	}
}

func TestLookupByAccountPropagatesError(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := lookupByAccount(context.Background(), testContext(), "boom")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("网络错误应原样上抛: err = %v", err)
	}
}

func TestSearchUsersKeyword(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	calls := withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status_code":1}`)
	})
	browser := &fakeBrowser{
		cookies: map[string]string{"sessionid": "login", "ttwid": "anon"},
		captured: []platform.CapturedResponse{
			{URL: "https://www.douyin.com/aweme/v1/web/general/search/single/", Body: string(fixture(t, "search_users_top.json"))},
			{URL: "https://www.douyin.com/aweme/v1/web/search/user/", Body: string(fixture(t, "search_users_wrapped.json"))},
		},
	}
	sc := testContext()
	sc.Browser = browser
	items, err := searchUsers(context.Background(), sc, "测试关键词")
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Errorf("关键词前应尝试一次查号, calls = %d", *calls)
	}
	assertSearchBrowser(t, browser, "测试关键词")
	assertSearchItems(t, items)
}

func assertSearchItems(t *testing.T, items []platform.Item) {
	t.Helper()
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	assertSearchItem(t, items[0], "MS4wLjABAAAA1234567890abcdef", "测试用户", "抖音号 test123 · 1.2万 粉丝")
	if items[0].Thumbnail == nil || *items[0].Thumbnail != "https://p3.douyinpic.com/thumb.jpg" {
		t.Errorf("thumbnail = %v", items[0].Thumbnail)
	}
	assertSearchItem(t, items[1], "MS4wLjABAAAA0987654321fedcba", "21fedcba", "")
	if items[1].Thumbnail != nil || items[1].Summary != nil {
		t.Errorf("空字段不应输出: %+v", items[1])
	}
	if items[2].Thumbnail == nil || *items[2].Thumbnail != "https://p3.douyinpic.com/wrap.jpg" {
		t.Errorf("medium 头像回退失败: %v", items[2].Thumbnail)
	}
	if items[3].Title != "空信息回落" {
		t.Errorf("user_info=null 应回落 entry: %q", items[3].Title)
	}
}

func assertSearchBrowser(t *testing.T, browser *fakeBrowser, keyword string) {
	t.Helper()
	if browser.origin != douyinOrigin || browser.cookiesURL != douyinOrigin {
		t.Errorf("origin/cookiesURL = %q/%q", browser.origin, browser.cookiesURL)
	}
	wantURL := douyinOrigin + "/search/" + encodeURIComponent(keyword) + "?type=user"
	if browser.captureURL != wantURL {
		t.Errorf("captureURL = %q, want %q", browser.captureURL, wantURL)
	}
	if browser.idle != captureIdle || browser.timeout != captureTimeout {
		t.Errorf("idle/timeout = %v/%v", browser.idle, browser.timeout)
	}
	if browser.closeCalls != 1 {
		t.Errorf("closeCalls = %d", browser.closeCalls)
	}
	searchAPI, _ := url.Parse("https://www.douyin.com/aweme/v1/web/general/search/single/")
	userPage, _ := url.Parse("https://www.douyin.com/user/MS4wLjABAAAAx")
	if browser.match == nil || !browser.match(searchAPI) || browser.match(userPage) {
		t.Error("match 判定不正确")
	}
}

func assertSearchItem(t *testing.T, item platform.Item, secUID, title, summary string) {
	t.Helper()
	if item.ID != "dyu-"+lastN(secUID, 12) || item.Title != title || item.SourceID != searchSourceID {
		t.Errorf("item = %+v", item)
	}
	if item.URL == nil || *item.URL != "https://www.douyin.com/user/"+secUID {
		t.Errorf("url = %v", item.URL)
	}
	if summary == "" {
		if item.Summary != nil {
			t.Errorf("summary = %q, want nil", *item.Summary)
		}
		return
	}
	if item.Summary == nil || *item.Summary != summary {
		t.Errorf("summary = %v, want %q", item.Summary, summary)
	}
}

func TestSearchUsersLoginGate(t *testing.T) {
	resetState()
	t.Cleanup(resetState)
	withUserInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("带空白的输入不应查号")
	})
	browser := &fakeBrowser{cookies: map[string]string{"ttwid": "anon"}}
	sc := testContext()
	sc.Browser = browser
	_, err := searchUsers(context.Background(), sc, "测试 关键词")
	if !errors.Is(err, platform.ErrBrowserLoginRequired) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "登录态与密钥") {
		t.Errorf("err = %v", err)
	}
	if browser.captureCalls != 0 || browser.closeCalls != 1 {
		t.Errorf("capture/close = %d/%d", browser.captureCalls, browser.closeCalls)
	}
}

func TestSearchUsersBrowserUnavailable(t *testing.T) {
	_, err := searchUsers(context.Background(), testContext(), "测试 关键词")
	if !errors.Is(err, platform.ErrBrowserUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchUsersNoResults(t *testing.T) {
	browser := &fakeBrowser{cookies: map[string]string{"sessionid": "login"}}
	sc := testContext()
	sc.Browser = browser
	_, err := searchUsers(context.Background(), sc, "测试 关键词")
	if err == nil || err.Error() != noResultsMessage {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchUsersCaptureError(t *testing.T) {
	browser := &fakeBrowser{
		cookies:    map[string]string{"sessionid": "login"},
		captureErr: errors.New("capture boom"),
	}
	sc := testContext()
	sc.Browser = browser
	_, err := searchUsers(context.Background(), sc, "测试 关键词")
	if err == nil || !strings.Contains(err.Error(), "capture boom") {
		t.Fatalf("err = %v", err)
	}
	if browser.closeCalls != 1 {
		t.Errorf("closeCalls = %d", browser.closeCalls)
	}
}

func TestSearchUsersCookiesError(t *testing.T) {
	browser := &fakeBrowser{cookiesErr: errors.New("cookies boom")}
	sc := testContext()
	sc.Browser = browser
	_, err := searchUsers(context.Background(), sc, "测试 关键词")
	if err == nil || !strings.Contains(err.Error(), "cookies boom") {
		t.Fatalf("err = %v", err)
	}
	if browser.captureCalls != 0 || browser.closeCalls != 1 {
		t.Errorf("capture/close = %d/%d", browser.captureCalls, browser.closeCalls)
	}
}
