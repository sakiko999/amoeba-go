// infobox 解析与「搜索/抓取共用」的取名规则:有效中文名 = 与原名不同的
// name_cn,否则 infobox 别名,否则原名。两处入口必须共用 pickTitle,
// 否则搜到的名字与写进 NFO 的对不上。

package bangumi

import (
	"encoding/json"
	"strings"
)

var aliasKeys = []string{"别名", "別名", "英文名"}

// infoboxValues 取指定 key 的所有非空值(字符串或 [{v:"..."}] 数组两种形态)。
func infoboxValues(items []rawInfoboxItem, keys []string) []string {
	out := []string{}
	for i := range items {
		item := &items[i]
		if item.Key == nil || !containsString(keys, strings.TrimSpace(*item.Key)) {
			continue
		}
		out = append(out, infoboxItemValues(item.Value)...)
	}
	return out
}

func infoboxItemValues(raw json.RawMessage) []string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil
	}
	if text[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil
		}
		return appendNonEmpty(nil, s)
	}
	if text[0] != '[' {
		return nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	out := []string{}
	for _, entry := range entries {
		var value struct {
			V *string `json:"v"`
		}
		if err := json.Unmarshal(entry, &value); err != nil || value.V == nil {
			continue
		}
		out = appendNonEmpty(out, *value.V)
	}
	return out
}

func appendNonEmpty(out []string, s string) []string {
	if t := strings.TrimSpace(s); t != "" {
		return append(out, t)
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// infoboxFirstAlias 第一个别名(无则空串)。
func infoboxFirstAlias(items []rawInfoboxItem) string {
	values := infoboxValues(items, aliasKeys)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// pickTitle 搜索与抓取共用的取名规则,返回标题与原名(与标题不同才给)。
func pickTitle(s *rawSubject) (string, *string) {
	nameCn := deref(s.NameCn)
	name := deref(s.Name)
	alias := infoboxFirstAlias(s.Infobox)
	cn := ""
	if nameCn != "" && nameCn != name {
		cn = nameCn
	} else if alias != "" && alias != name {
		cn = alias
	}
	title := name
	if cn != "" {
		title = cn
	}
	var original *string
	if name != "" && name != title {
		original = &name
	}
	return title, original
}

// isIsoDate 宽松校验 yyyy-mm-dd(date 偶尔只有年份或空串)。
func isIsoDate(s string) bool {
	return len(s) == 10 && s[4] == '-' && s[7] == '-'
}
