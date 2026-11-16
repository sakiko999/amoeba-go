// 移植 TS example/hot-channels.check.ts 的 bili 部分：热搜词条必须是 social（条目级），
// 名字带「热搜/热点」的渠道不得声明成 article。

package bili

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestSquareItems(t *testing.T) {
	list := []any{
		map[string]any{"keyword": "AI", "link": "https://search.bilibili.com/all?keyword=AI", "icon": "https://i0.hdslb.com/x.jpg"},
		map[string]any{"keyword": "没有链接"},
	}
	items := squareItems(list, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	for _, item := range items {
		if item.Kind != "social" {
			t.Errorf("%s kind = %v, want social", item.ID, item.Kind)
		}
		if item.Content == nil || strings.ContainsAny(*item.Content, "<>") {
			t.Errorf("正文必须是纯文本: %v", item.Content)
		}
	}
	if items[0].Thumbnail == nil || *items[0].Thumbnail != "https://i0.hdslb.com/x.jpg" {
		t.Errorf("icon → thumbnail = %v", items[0].Thumbnail)
	}
	if items[1].URL == nil || !strings.Contains(*items[1].URL, "search.bilibili.com") {
		t.Errorf("缺 link 时应用关键词拼搜索地址: %v", items[1].URL)
	}
	if items[0].ID != "bili-rank-AI" {
		t.Errorf("id = %q, want bili-rank-AI", items[0].ID)
	}
	if items[1].Thumbnail != nil {
		t.Error("非字符串 icon 不应产出 thumbnail")
	}
	var raw map[string]any
	if err := json.Unmarshal(items[0].Raw, &raw); err != nil || raw["keyword"] != "AI" {
		t.Errorf("raw 透传失败: %v / %v", err, raw)
	}
}

func TestHotKeywordChannelsAreSocial(t *testing.T) {
	pattern := regexp.MustCompile(`热搜|热点`)
	found := 0
	for _, c := range testChannels(t) {
		if !pattern.MatchString(c.Name()) {
			continue
		}
		found++
		if c.Kind() != "social" {
			t.Errorf("%s(%s) kind = %s, want social", c.Key(), c.Name(), c.Kind())
		}
	}
	if found == 0 {
		t.Error("未识别到热搜/热点渠道")
	}
}
