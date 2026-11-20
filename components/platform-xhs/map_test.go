// 用户投影映射与粉丝数缩写的纯函数测试(不碰网络/载具)。

package xhs

import "testing"

func TestFmtCount(t *testing.T) {
	cases := []struct {
		n    float64
		want string
	}{
		{0, "0"},
		{9999, "9999"},
		{10000, "1万"},
		{12345, "1.2万"},
		{99999999, "10000万"},
		{100000000, "1亿"},
		{120000000, "1.2亿"},
		{1234.5, "1234.5"},
	}
	for _, tc := range cases {
		if got := fmtCount(tc.n); got != tc.want {
			t.Errorf("fmtCount(%v) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestProjectUsers(t *testing.T) {
	hits := projectUsers([]projectedUser{
		{UserID: "u1", Nickname: "A", Avatar: "a.jpg", RedID: "r1", Fans: 12345},
		{UserID: "u2", Nickname: "B"},
	})
	if len(hits) != 2 {
		t.Fatalf("hits = %d", len(hits))
	}
	if hits[0].Desc != "小红书号 r1" || hits[0].DescExtra != "1.2万 粉丝" {
		t.Errorf("hits[0] = %+v", hits[0])
	}
	if hits[1].Desc != "" || hits[1].DescExtra != "" {
		t.Errorf("hits[1] = %+v", hits[1])
	}
}

func TestDedupeUsers(t *testing.T) {
	list := []xhsUserHit{
		{UserID: "u1", Nickname: "A", Avatar: "first"},
		{UserID: "u1", Nickname: "A2", Avatar: "second"},
		{UserID: "", Nickname: "C"},
		{UserID: "u3", Nickname: ""},
		{UserID: "u2", Nickname: "B"},
	}
	out := dedupeUsers(list)
	if len(out) != 2 {
		t.Fatalf("out = %+v", out)
	}
	if out[0].UserID != "u1" || out[0].Avatar != "first" {
		t.Errorf("out[0] = %+v, want 保留首次出现", out[0])
	}
	if out[1].UserID != "u2" {
		t.Errorf("out[1] = %+v", out[1])
	}
}

func TestUserToItem(t *testing.T) {
	item := userToItem(xhsUserHit{
		UserID: "u1", Nickname: "美食博主", Avatar: "https://img/x.jpg",
		Desc: "小红书号 r1", DescExtra: "1.2万 粉丝",
	}, testNow)
	if item.ID != "xu-u1" || item.SourceID != sourceID || item.Kind != "video" {
		t.Errorf("item = %+v", item)
	}
	if item.Title != "美食博主" || item.FetchedAt != testNow {
		t.Errorf("item title/at = %s/%d", item.Title, item.FetchedAt)
	}
	if item.URL == nil || *item.URL != origin+"/user/profile/u1" {
		t.Errorf("item url = %v", item.URL)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://img/x.jpg" {
		t.Errorf("item thumbnail = %v", item.Thumbnail)
	}
	if item.Summary == nil || *item.Summary != "小红书号 r1 · 1.2万 粉丝" {
		t.Errorf("item summary = %v", item.Summary)
	}
	if item.Author == nil || item.Author.Name != "美食博主" || item.Author.Handle == nil || *item.Author.Handle != "u1" {
		t.Errorf("item author = %+v", item.Author)
	}

	minimal := userToItem(xhsUserHit{UserID: "u2", Nickname: "旅行者"}, testNow)
	if minimal.Thumbnail != nil || minimal.Summary != nil {
		t.Errorf("minimal = %+v, want 省略可选字段", minimal)
	}
}

func TestSummaryOf(t *testing.T) {
	cases := []struct {
		hit  xhsUserHit
		want string
	}{
		{xhsUserHit{Desc: "小红书号 r1"}, "小红书号 r1"},
		{xhsUserHit{DescExtra: "3 粉丝"}, "3 粉丝"},
		{xhsUserHit{Desc: "小红书号 r1", DescExtra: "3 粉丝"}, "小红书号 r1 · 3 粉丝"},
		{xhsUserHit{}, ""},
	}
	for _, tc := range cases {
		got := summaryOf(tc.hit)
		if tc.want == "" {
			if got != nil {
				t.Errorf("summaryOf(%+v) = %q, want nil", tc.hit, *got)
			}
			continue
		}
		if got == nil || *got != tc.want {
			t.Errorf("summaryOf(%+v) = %v, want %q", tc.hit, got, tc.want)
		}
	}
}

func TestUserToItemNicknameFallback(t *testing.T) {
	item := userToItem(xhsUserHit{Nickname: "无 ID"}, testNow)
	if item.ID != "xu-无 ID" {
		t.Errorf("id = %q, want 昵称兜底", item.ID)
	}
	if item.URL == nil || *item.URL != origin+"/user/profile/" {
		t.Errorf("url = %v", item.URL)
	}
}

func TestSearchURLEncoding(t *testing.T) {
	got := searchURL("a b&c")
	want := origin + "/search_result?keyword=a%20b%26c&source=web_search_result_notes&type=51"
	if got != want {
		t.Errorf("searchURL = %q, want %q", got, want)
	}
}
