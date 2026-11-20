// getIndex 解析语义测试:q 过滤、首个 ok=1、-100 风控、卡片遍历与字段归一。

package weibo

import (
	"testing"

	"mygo-host/platform"
)

func TestParseFiltersByKeyword(t *testing.T) {
	keyword := "半佛仙人"
	responses := []platform.CapturedResponse{
		{URL: capturedURL(userContainer("其他关键词")), Body: okCards(`{"user":{"id":111,"screen_name":"串台用户"}}`)},
		{URL: "http://[::1", Body: okCards(`{"user":{"id":112,"screen_name":"坏 URL"}}`)},
		{URL: capturedURL(userContainer(keyword)), Body: `{broken`},
		{URL: capturedURL(userContainer(keyword)), Body: `{"ok":-100}`},
		{URL: capturedURL(userContainer(keyword)), Body: okCards(`{"user":{"id":123,"screen_name":"命中用户"}}`)},
	}
	hits, found, risk := parseUserSearch(responses, keyword)
	if !found || !risk {
		t.Fatalf("found/risk = %v/%v, want true/true", found, risk)
	}
	if len(hits) != 1 || hits[0].UID != "123" || hits[0].ScreenName != "命中用户" {
		t.Fatalf("hits = %+v, want 仅 q=关键词 的命中用户", hits)
	}
}

func TestParseIncludesQuerySubstring(t *testing.T) {
	// includes 而非 q 值精确相等:containerid 内可能追加额外参数。
	responses := []platform.CapturedResponse{{
		URL:  capturedURL("100103type=3&q=半佛仙人&page=2"),
		Body: okCards(`{"user":{"id":1,"screen_name":"甲"}}`),
	}}
	hits, found, _ := parseUserSearch(responses, "半佛仙人")
	if !found || len(hits) != 1 {
		t.Fatalf("hits = %+v found = %v, want includes 命中", hits, found)
	}
}

func TestParseFirstOKWins(t *testing.T) {
	responses := []platform.CapturedResponse{
		{URL: capturedURL(userContainer("kw")), Body: okCards(`{"user":{"id":1,"screen_name":"第一个"}}`)},
		{URL: capturedURL(userContainer("kw")), Body: okCards(`{"user":{"id":2,"screen_name":"第二个"}}`)},
	}
	hits, found, _ := parseUserSearch(responses, "kw")
	if !found || len(hits) != 1 || hits[0].ScreenName != "第一个" {
		t.Fatalf("hits = %+v, want 只取首个 ok=1", hits)
	}
}

func TestParseOnlyNumericOKCounts(t *testing.T) {
	bodies := []string{
		`{"ok":"1","data":{"cards":[{"user":{"id":1,"screen_name":"甲"}}]}}`,
		`{"ok":true,"data":{"cards":[{"user":{"id":1,"screen_name":"甲"}}]}}`,
		`{"ok":null,"data":{"cards":[{"user":{"id":1,"screen_name":"甲"}}]}}`,
	}
	for _, body := range bodies {
		hits, found, risk := parseUserSearch([]platform.CapturedResponse{{
			URL: capturedURL(userContainer("kw")), Body: body,
		}}, "kw")
		if found || risk || hits != nil {
			t.Errorf("body %s → found/risk/hits = %v/%v/%v, want 全空", body, found, risk, hits)
		}
	}
}

func TestHitsFromCardsSemantics(t *testing.T) {
	body := `{"ok":1,"data":{"cards":[
		{"card_type":4},
		{"card_group":[
			{"user":{"id":1,"screen_name":"甲","avatar_hd":"","profile_image_url":"p1","followers_count":0,"verified_reason":"  认证理由  "}},
			{"user":{"id":1,"screen_name":"甲的重复","profile_image_url":"p2"}},
			{"user":{"id":2,"screen_name":"   "}}
		]},
		{"user":{"id":0,"screen_name":"零号"}},
		{"user":{"id":3,"screen_name":"单体卡","profile_image_url":"p3","followers_count":"207.7万"}},
		{"card_group":[],"user":{"id":4,"screen_name":"组卡为空时忽略"}}
	]}}`
	hits, found, _ := parseUserSearch([]platform.CapturedResponse{{
		URL: capturedURL(userContainer("kw")), Body: body,
	}}, "kw")
	if !found {
		t.Fatal("found = false")
	}
	if len(hits) != 3 {
		t.Fatalf("hits = %+v, want 3 条", hits)
	}
	first := hits[0]
	if first.UID != "1" || first.ScreenName != "甲" || first.Avatar != "" ||
		first.Followers != "0" || first.Verified != "认证理由" {
		t.Errorf("hits[0] = %+v", first)
	}
	if hits[1].UID != "0" || hits[1].ScreenName != "零号" {
		t.Errorf("hits[1] = %+v, want uid 0 保留", hits[1])
	}
	if hits[2].Avatar != "p3" || hits[2].Followers != "207.7万" {
		t.Errorf("hits[2] = %+v, want avatar_hd 缺失时回退 + 保留格式化粉丝数", hits[2])
	}
}

func TestParseDuplicateUIDKeepsFirst(t *testing.T) {
	body := okCards(
		`{"user":{"id":7,"screen_name":"先说"}},` +
			`{"card_group":[{"user":{"id":7,"screen_name":"后说","profile_image_url":"p"}}]}`)
	hits, _, _ := parseUserSearch([]platform.CapturedResponse{{
		URL: capturedURL(userContainer("kw")), Body: body,
	}}, "kw")
	if len(hits) != 1 || hits[0].ScreenName != "先说" || hits[0].Avatar != "" {
		t.Fatalf("hits = %+v, want 首次出现决定最终值", hits)
	}
}

func okCards(cards string) string {
	return `{"ok":1,"data":{"cards":[` + cards + `]}}`
}
