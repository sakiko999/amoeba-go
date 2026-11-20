// 被动捕获解析测试:结果包路径、精确形状、深遍历兜底与 hasResultPack 分支。

package xhs

import (
	"net/url"
	"testing"

	"mygo-host/platform"
)

func TestIsXhsSearchAPI(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://www.xiaohongshu.com/api/sns/web/v1/search/usersearch", true},
		{"https://edith.xiaohongshu.com/api/sns/web/v1/search/notes", true},
		{"https://xiaohongshu.com/api/sns/web/v1/search/onebox", true},
		{"https://xiaohongshu.com/search_result?keyword=x", false},
		{"https://evil-xiaohongshu.com/api/sns/web/v1/search/usersearch", false},
		{"https://xiaohongshu.com.evil.com/api/sns/web/v1/search/usersearch", false},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := isXhsSearchAPI(u); got != tc.want {
			t.Errorf("isXhsSearchAPI(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestCapturedUsersExactShapes(t *testing.T) {
	responses := []platform.CapturedResponse{
		{URL: "https://edith.xiaohongshu.com/api/sns/web/v1/search/usersearch",
			Body: `{"data":{"users":[{"id":"u1","nickname":"精确用户","red_id":"r1","fans":12345}]}}`},
		{URL: "https://edith.xiaohongshu.com/api/sns/web/v1/search/notes",
			Body: `{"data":{"items":[{"note_card":{"user":{"user_id":"u2","name":"笔记作者"}}}]}}`},
		{URL: "https://edith.xiaohongshu.com/api/sns/web/v1/search/usersearch",
			Body: `{"data":{"data":{"users":[{"user_id":"u3","nickname":"嵌套用户"}]}}}`},
	}
	hits, hasPack := capturedUsers(responses)
	if !hasPack {
		t.Fatal("hasResultPack 应为 true")
	}
	if len(hits) != 3 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].UserID != "u1" || hits[0].Desc != "小红书号 r1" || hits[0].DescExtra != "1.2万 粉丝" {
		t.Errorf("hits[0] = %+v", hits[0])
	}
	if hits[1].UserID != "u2" || hits[1].Nickname != "笔记作者" {
		t.Errorf("hits[1] = %+v", hits[1])
	}
	if hits[2].UserID != "u3" {
		t.Errorf("hits[2] = %+v", hits[2])
	}
}

// TestCapturedUsersWalkFallback 精确解析不到用户时,只对 usersearch/user 包深遍历。
func TestCapturedUsersWalkFallback(t *testing.T) {
	body := `{"data":{"recommend":{"cards":[{"id":"u7","nickname":"深层用户","fans":3}]}}}`
	hits, hasPack := capturedUsers([]platform.CapturedResponse{{
		URL:  "https://edith.xiaohongshu.com/api/sns/web/v1/search/user",
		Body: body,
	}})
	if !hasPack || len(hits) != 1 || hits[0].UserID != "u7" {
		t.Fatalf("hits/hasPack = %+v/%v", hits, hasPack)
	}
}

func TestCapturedUsersResultPackWithoutUsers(t *testing.T) {
	hits, hasPack := capturedUsers([]platform.CapturedResponse{{
		URL:  "https://edith.xiaohongshu.com/api/sns/web/v1/search/usersearch",
		Body: `{"data":{"users":[]}}`,
	}})
	if len(hits) != 0 || !hasPack {
		t.Fatalf("hits/hasPack = %+v/%v, want 0/true", hits, hasPack)
	}
}

func TestCapturedUsersNoResultPackWalksAll(t *testing.T) {
	hits, hasPack := capturedUsers([]platform.CapturedResponse{{
		URL:  "https://edith.xiaohongshu.com/api/sns/web/v1/search/onebox",
		Body: `{"data":{"users":[{"user_id":"u8","nickname":"onebox 用户","red_id":"r8"}]}}`,
	}})
	if hasPack || len(hits) != 1 || hits[0].UserID != "u8" {
		t.Fatalf("hits/hasPack = %+v/%v, want 1/false", hits, hasPack)
	}
}

func TestUserNodeToHitFilters(t *testing.T) {
	if _, ok := userNodeToHit(map[string]any{
		"id": "note1", "nickname": "笔记", "note_id": "n1",
	}); ok {
		t.Error("含 note_id 的对象应排除")
	}
	if _, ok := userNodeToHit(map[string]any{
		"id": "plain1", "nickname": "非 userish",
	}); ok {
		t.Error("userish 为假时不应把 id 当 userId")
	}
	hit, ok := userNodeToHit(map[string]any{
		"_rawValue": map[string]any{"id": "u5", "nickname": "包装用户", "fans": 1},
	})
	if !ok || hit.UserID != "u5" || hit.DescExtra != "1 粉丝" {
		t.Errorf("hit = %+v/%v", hit, ok)
	}
	obj, ok := userNodeToHit(map[string]any{
		"user_id": "u6", "nickname": "对象头像", "image": map[string]any{"url": "x"},
	})
	if !ok || obj.Avatar != "[object Object]" {
		t.Errorf("capture avatar = %q, want 对象陷阱原样", obj.Avatar)
	}
}

func TestParseFans(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{12345.0, 12345},
		{"29.9万", 299000},
		{"3.4亿", 340000000},
		{"10万+", 0},
		{"1,234", 0},
		{"", 0},
		{map[string]any{}, 0},
	}
	for _, tc := range cases {
		if got := parseFans(tc.in); got != tc.want {
			t.Errorf("parseFans(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
