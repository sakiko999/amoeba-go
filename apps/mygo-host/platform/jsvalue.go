// JS 值语义辅助(对齐 platform-kit runtime/utils 的 str.ts/number.ts 共享层):
// 上游 JSON 字段形态不定(数字/数字串/空串/缺失),映射层统一经这里按 TS 的
// String() / Number() / truthiness / ?? 语义归一。各平台特有变体
// (如 bili 的 nil→0 数值、youtube 的解析失败→0)留在各包,不上收。

package platform

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// JSString 等价 JS String(v);null / undefined 归一为空串。
func JSString(v any) string {
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

// jsNumberString 按 JS Number#toString 的常用形态格式化(≥1e21 走科学计数法)。
func jsNumberString(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		return "0"
	}
	if math.Trunc(f) == f && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// JSNumber 等价 JS Number(v);ok=false 等价 NaN。
func JSNumber(v any) (float64, bool) {
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

// ToInt 等价 TS toInt:null/undefined/空串/非有限 → 未命中。
func ToInt(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	if s, ok := v.(string); ok && s == "" {
		return 0, false
	}
	n, ok := JSNumber(v)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// JSTruthy 等价 JS 真值判断(对象/数组恒真)。
func JSTruthy(v any) bool {
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

// StrOr 等价 TS strOr:null/undefined/空串 → nil,其余 String 化。
func StrOr(v any) *string {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return nil
		}
		return Str(s)
	}
	return Str(JSString(v))
}
