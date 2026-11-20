// Item 映射黄金集与纯函数(粉丝数缩写/末 N 字符)测试。

package douyin

import (
	"testing"

	"mygo-host/platform"
)

func TestUserItem(t *testing.T) {
	item := userItem(userHit{
		secUID:       "MS4wLjABAAAA1234567890",
		nickname:     "测试用户",
		avatar:       "https://p3.douyinpic.com/a.jpg",
		uniqueID:     "test123",
		followers:    12345,
		hasFollowers: true,
	}, testNow)
	if item.ID != "dyu-AA1234567890" || item.SourceID != searchSourceID || item.Kind != platform.KindVideo {
		t.Errorf("item = %+v", item)
	}
	if item.Title != "测试用户" || item.FetchedAt != testNow {
		t.Errorf("title/fetchedAt = %q/%d", item.Title, item.FetchedAt)
	}
	if item.URL == nil || *item.URL != "https://www.douyin.com/user/MS4wLjABAAAA1234567890" {
		t.Errorf("url = %v", item.URL)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://p3.douyinpic.com/a.jpg" {
		t.Errorf("thumbnail = %v", item.Thumbnail)
	}
	if item.Summary == nil || *item.Summary != "抖音号 test123 · 1.2万 粉丝" {
		t.Errorf("summary = %v", item.Summary)
	}
	if item.Author == nil || item.Author.Name != "测试用户" || item.Author.Handle == nil ||
		*item.Author.Handle != "MS4wLjABAAAA1234567890" || item.Author.Avatar == nil {
		t.Errorf("author = %+v", item.Author)
	}
}

func TestUserItemOmitsEmptyFields(t *testing.T) {
	item := userItem(userHit{secUID: "MS4wLjABAAAAx", nickname: "n"}, testNow)
	if item.Thumbnail != nil || item.Summary != nil || item.Author.Avatar != nil {
		t.Errorf("空字段不应输出: %+v", item)
	}
	if item.Author == nil || item.Author.Name != "n" || item.Author.Handle == nil {
		t.Errorf("author = %+v", item.Author)
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{9999, "9999"},
		{10000, "1万"},
		{12345, "1.2万"},
		{15000, "1.5万"},
		{99999999, "10000万"},
		{100000000, "1亿"},
		{128000000, "1.3亿"},
	}
	for _, tc := range cases {
		if got := formatCount(tc.in); got != tc.want {
			t.Errorf("formatCount(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLastN(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abc", 2, "bc"},
		{"中文昵称", 2, "昵称"},
		{"abc", 10, "abc"},
		{"abc", 0, ""},
	}
	for _, tc := range cases {
		if got := lastN(tc.in, tc.n); got != tc.want {
			t.Errorf("lastN(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}
