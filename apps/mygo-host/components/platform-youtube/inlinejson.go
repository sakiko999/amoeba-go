// 内联 JSON 提取(对齐 platform-kit runtime/utils/inline-json):从页面 script 里
// 按平衡大括号截取 JSON 对象,字符串内的括号与转义跳过;返回原始文本供按需解码。

package youtube

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractInlineJSON(html, marker, label string) (string, error) {
	markerIdx := strings.Index(html, marker)
	if markerIdx < 0 {
		return "", fmt.Errorf("%s: 未找到 %s", label, marker)
	}
	start := strings.Index(html[markerIdx:], "{")
	if start < 0 {
		return "", fmt.Errorf("%s: %s 块未找到", label, marker)
	}
	start += markerIdx
	end := balancedJSONEnd(html, start)
	if end < 0 {
		return "", fmt.Errorf("%s: 括号不平衡", label)
	}
	raw := html[start : end+1]
	if !json.Valid([]byte(raw)) {
		return "", fmt.Errorf("%s: JSON 解析失败", label)
	}
	return raw, nil
}

func balancedJSONEnd(s string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
