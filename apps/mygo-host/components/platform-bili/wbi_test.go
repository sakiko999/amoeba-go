// 移植 TS example/bili-space-sign.check.ts 与 bili-buvid.check.ts：
// wbi 签名黄金值（web / 直播两套语义）、space 指纹签名、buvid 兜底与 code 错误。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const (
	testImgKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testSubKey = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func newWbiClient(t *testing.T) (*client, *fakeServer) {
	t.Helper()
	fs := &fakeServer{respond: navResponder(testImgKey, testSubKey, nil)}
	return newClient(newTestContext(t, fs, testNow)), fs
}

func TestSignWebGolden(t *testing.T) {
	c, _ := newWbiClient(t)
	ctx := context.Background()
	cases := []struct{ query, want string }{
		{"limit=50&platform=web", "limit=50&platform=web&w_rid=2252532b69a6a4fbe12aef26ac9a388d&wts=1700000000"},
		{
			"mid=42&ps=30&pn=1&platform=web&order=pubdate&keyword=&tid=0&web_location=1550101",
			"mid=42&ps=30&pn=1&platform=web&order=pubdate&keyword=&tid=0&web_location=1550101&w_rid=608465b56d42a77ecdb79a349438e63a&wts=1700000000",
		},
		{"keyword=半佛 仙人&page=1", "keyword=半佛 仙人&page=1&w_rid=4d53333e827ed607b6cc5991025fea1c&wts=1700000000"},
	}
	for _, tc := range cases {
		got, err := c.signWeb(ctx, tc.query, nil)
		if err != nil {
			t.Fatalf("signWeb(%q): %v", tc.query, err)
		}
		if got != tc.want {
			t.Errorf("signWeb(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestSignLiveParamsGolden(t *testing.T) {
	c, _ := newWbiClient(t)
	ctx := context.Background()
	got, err := c.signLiveParams(ctx, map[string]string{
		"room_id": "6", "protocol": "0,1", "format": "0,1,2", "codec": "0,1", "platform": "web", "qn": "10000",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "codec=0%2C1&format=0%2C1%2C2&platform=web&protocol=0%2C1&qn=10000&room_id=6&wts=1700000000&w_rid=e8e61c4c9136c8d3402cc21d01e1a1d7"
	if got != want {
		t.Errorf("signLiveParams = %q, want %q", got, want)
	}
	got, err = c.signLiveParams(ctx, map[string]string{"room_id": "24022841"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want = "room_id=24022841&wts=1700000000&w_rid=8749878531dcd898c45d363d1fc6a095"
	if got != want {
		t.Errorf("signLiveParams(单房间) = %q, want %q", got, want)
	}
}

func TestSignSpaceFingerprint(t *testing.T) {
	c, fs := newWbiClient(t)
	ctx := context.Background()
	signed, err := c.signSpace(ctx, "mid=42&ps=30&pn=1&platform=web&order=pubdate", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dm_img_list=", "dm_img_str=", "dm_cover_img_str=", "dm_img_inter=", "w_rid=", "wts="} {
		mustContain(t, signed, key, "signSpace 输出")
	}
	pairs := parseFormPairs(signed)
	wts, wRid := "", ""
	rest := make([]formPair, 0, len(pairs))
	for _, p := range pairs {
		switch p.name {
		case "wts":
			wts = p.value
		case "w_rid":
			wRid = p.value
		default:
			rest = append(rest, p)
		}
	}
	if wts != "1700000000" {
		t.Errorf("wts = %q, want 1700000000", wts)
	}
	mixin := testMixinKey()
	want := md5Hex(sortedFormEncode(rest) + "&wts=" + wts + mixin)
	if wRid != want {
		t.Errorf("w_rid 未覆盖含 dm_* 的全参数: got %q want %q", wRid, want)
	}

	// 带 cookie 时 nav 必须用同一 cookie（登录态 key 选择）。
	fs.reset()
	if _, err := c.signSpace(ctx, "mid=42", pstr("SESSDATA=abc")); err != nil {
		t.Fatal(err)
	}
	nav := fs.find("/x/web-interface/nav")
	if nav == nil || nav.Header.Get("cookie") != "SESSDATA=abc" {
		t.Fatalf("nav cookie = %v, want SESSDATA=abc", nav)
	}
}

func testMixinKey() string {
	origin := testImgKey + testSubKey
	var b strings.Builder
	for _, i := range mixinKeyEncTab {
		if i < len(origin) {
			b.WriteByte(origin[i])
		}
	}
	key := b.String()
	if len(key) > 32 {
		key = key[:32]
	}
	return key
}

func TestBuvidFallback(t *testing.T) {
	const areaURL = "https://api.live.bilibili.com/xlive/web-interface/v1/second/getListByArea"
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/frontend/finger/spi") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"b_3":"ANON_B3","b_4":"ANON_B4"}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
	}}
	c := newClient(newTestContext(t, fs, testNow))
	ctx := context.Background()

	// 1. 登录 cookie 无 buvid3 → 补匿名指纹且保留登录凭证。
	if _, _, err := c.getJSON(ctx, areaURL, reqOptions{Cookie: pstr("SESSDATA=abc; bili_jct=xyz"), Buvid: true}); err != nil {
		t.Fatal(err)
	}
	cookie := fs.find("second/getListByArea").Header.Get("cookie")
	mustContain(t, cookie, "SESSDATA=abc", "登录 cookie")
	mustContain(t, cookie, "buvid3=ANON_B3", "补匿名 buvid3")
	mustContain(t, cookie, "buvid4=ANON_B4", "补匿名 buvid4")

	// 2. 已带 buvid3 → 不重复追加。
	fs.reset()
	if _, _, err := c.getJSON(ctx, areaURL, reqOptions{Cookie: pstr("SESSDATA=abc; buvid3=OWN_B3; buvid4=OWN_B4"), Buvid: true}); err != nil {
		t.Fatal(err)
	}
	cookie = fs.find("second/getListByArea").Header.Get("cookie")
	mustContain(t, cookie, "buvid3=OWN_B3", "尊重真实指纹")
	if strings.Contains(cookie, "ANON_B3") {
		t.Errorf("已有 buvid3 时不得再补匿名指纹: %q", cookie)
	}

	// 3. 无 cookie → 仍补匿名指纹。
	fs.reset()
	if _, _, err := c.getJSON(ctx, areaURL, reqOptions{Buvid: true}); err != nil {
		t.Fatal(err)
	}
	if cookie = fs.find("second/getListByArea").Header.Get("cookie"); cookie != "buvid3=ANON_B3;buvid4=ANON_B4;" {
		t.Errorf("匿名 buvid cookie = %q", cookie)
	}
}

func TestBuvidEmptyResultCached(t *testing.T) {
	spiCalls := 0
	fs := &fakeServer{respond: func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/frontend/finger/spi") {
			spiCalls++
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"b_3":"","b_4":""}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
	}}
	c := newClient(newTestContext(t, fs, testNow))
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, _, err := c.getJSON(ctx, apiMain+"/x/test", reqOptions{Buvid: true}); err != nil {
			t.Fatal(err)
		}
	}
	if spiCalls != 1 {
		t.Errorf("空 buvid 结果也应缓存,spi 请求数 = %d, want 1", spiCalls)
	}
}

func TestGetJSONCodeError(t *testing.T) {
	fs := &fakeServer{respond: func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":-352,"message":"风控校验失败"}`), nil
	}}
	c := newClient(newTestContext(t, fs, testNow))
	ctx := context.Background()
	_, _, err := c.getJSON(ctx, apiMain+"/x/test", reqOptions{})
	if err == nil || err.Error() != "bilibili API -352: 风控校验失败" {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := c.getJSON(ctx, apiMain+"/x/test", reqOptions{AllowCodeError: true}); err != nil {
		t.Fatalf("allowCodeError 不应报错: %v", err)
	}
}

func pstr(s string) *string { return &s }
