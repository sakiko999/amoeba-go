// douyu 特有辅助:parseCount(「万/w/k」计数归一)、roomId 列表解析、
// encodeURIComponent、HTML 反转义。JS 值语义归一(jsString/toInt/truthy/
// strOr)在 platform/jsvalue.go 共享。

package douyu

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"mygo-host/platform"
)

var countRe = regexp.MustCompile(`(?i)^([\d.]+)\s*(万|w|k)?`)

// jsNumberOr 等价 Number(v ?? fallback):缺失取 fallback,非数值 → NaN。
func jsNumberOr(v any, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	n, ok := platform.JSNumber(v)
	if !ok {
		return math.NaN()
	}
	return n
}

// parseCount 等价 TS parseCount:数字直通,其余按「万/w/k」单位换算。
func parseCount(v any) *int {
	switch value := v.(type) {
	case nil:
		return nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}
		return platform.Int(int(value))
	case int:
		return platform.Int(value)
	case json.Number:
		n, err := value.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil
		}
		return platform.Int(int(n))
	}
	m := countRe.FindStringSubmatch(platform.JSString(v))
	if len(m) < 2 {
		return nil
	}
	n, ok := parseFloatPrefix(m[1])
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	unit := strings.ToLower(m[2])
	switch unit {
	case "万", "w":
		return platform.Int(int(math.Round(n * 10000)))
	case "k":
		return platform.Int(int(math.Round(n * 1000)))
	default:
		return platform.Int(int(n))
	}
}

// parseFloatPrefix 等价 JS parseFloat 对「数字/点」串的前缀解析
// ("1.2.3" → 1.2;第二个点处停止)。
func parseFloatPrefix(s string) (float64, bool) {
	seenDot := false
	end := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if seenDot {
				return parsePrefixValue(s[:end])
			}
			seenDot = true
			end = i + 1
			continue
		}
		if c < '0' || c > '9' {
			return parsePrefixValue(s[:end])
		}
		end = i + 1
	}
	return parsePrefixValue(s[:end])
}

func parsePrefixValue(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseRoomIds 等价 TS parseRoomIds:info.roomIds 逗号/中文逗号/空白分隔。
func parseRoomIds(info platform.SourceInfo) []string {
	raw := platform.JSString(info["roomIds"])
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || unicode.IsSpace(r)
	})
	if parts == nil {
		return []string{}
	}
	return parts
}

// encodeURIComponent 等价 JS encodeURIComponent(UTF-8 逐字节)。
func encodeURIComponent(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '!' || c == '~' || c == '*' || c == '\'' || c == '(' || c == ')' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

// htmlUnescape 对齐 TS 的连续 replace 顺序(&amp; 先于 &lt; 等)。
func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.ReplaceAll(s, "&#x27;", "'")
}
