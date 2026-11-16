// douyu 的 JS 语义辅助:上游 JSON 字段形态不定(数字/数字串/空串/缺失),
// 映射层按 TS 的 String() / Number() / truthiness / ?? 语义归一。
// 这些 helper 只服务本平台,不放进共享运行时。

package douyu

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"mygo-host/platform"
)

var countRe = regexp.MustCompile(`(?i)^([\d.]+)\s*(万|w|k)?`)

// jsString 等价 JS String(v);null / undefined 归一为空串。
func jsString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		if value {
			return "true"
		}
		return "false"
	case float64:
		return jsNumberString(value)
	case float32:
		return jsNumberString(float64(value))
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case json.Number:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

// jsNumberString 按 JS Number#toString 的常用形态格式化。
func jsNumberString(f float64) string {
	if f == 0 {
		return "0"
	}
	if math.Trunc(f) == f && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// jsNumber 等价 JS Number(v);ok=false 等价 NaN。
func jsNumber(v any) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, !math.IsNaN(value)
	case float32:
		n := float64(value)
		return n, !math.IsNaN(n)
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		n, err := value.Float64()
		return n, err == nil && !math.IsNaN(n)
	case string:
		s := strings.TrimSpace(value)
		if s == "" {
			return 0, true
		}
		n, err := strconv.ParseFloat(s, 64)
		return n, err == nil && !math.IsNaN(n)
	case bool:
		if value {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// jsNumberOr 等价 Number(v ?? fallback):缺失取 fallback,非数值 → NaN。
func jsNumberOr(v any, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	n, ok := jsNumber(v)
	if !ok {
		return math.NaN()
	}
	return n
}

// toInt 等价 TS toInt:null/undefined/空串/非有限 → 未命中。
func toInt(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	if s, ok := v.(string); ok && s == "" {
		return 0, false
	}
	n, ok := jsNumber(v)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// truthy 等价 JS 真值判断(对象/数组恒真)。
func truthy(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0 && !math.IsNaN(value)
	case float32:
		n := float64(value)
		return n != 0 && !math.IsNaN(n)
	case int:
		return value != 0
	case int64:
		return value != 0
	case json.Number:
		n, err := value.Float64()
		return err == nil && n != 0 && !math.IsNaN(n)
	default:
		return true
	}
}

// strOr 等价 TS strOr:null/undefined/空串 → nil,其余 String 化。
func strOr(v any) *string {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return nil
		}
		return platform.Str(s)
	}
	return platform.Str(jsString(v))
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
	m := countRe.FindStringSubmatch(jsString(v))
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
	raw := jsString(info["roomIds"])
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
