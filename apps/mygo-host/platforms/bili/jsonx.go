// JS 值语义的 JSON 辅助：B 站上游响应大量 optional 字段，映射层统一经这里取值，
// 行为对齐 TS 的 String() / Number() / 真值判断（数字格式化不能走 fmt.Sprint，
// 否则 24022841 会变成 2.4022841e+07）。

package bili

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// jobj 取对象（类型不符返回 nil）。
func jobj(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// jlist 取数组（类型不符返回 nil）。
func jlist(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

// jstr 等价 JS String(v ?? "")：nil → ""，数字按 JS 十进制形态输出。
func jstr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return jsNum(x)
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}

// jstrOr 等价 String(v ?? fallback)（仅 null/undefined 回落）。
func jstrOr(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	return jstr(v)
}

// jsNum 按 JS Number→String 规则格式化（普通十进制，不用科学计数法）。
func jsNum(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jnum 等价 JS Number(v)：数字原样，数字字符串解析，其余 NaN（nil → 0，对应 ?? 0）。
func jnum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return math.NaN()
		}
		return f
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return math.NaN()
		}
		return f
	case bool:
		if x {
			return 1
		}
		return 0
	case nil:
		return 0
	}
	return math.NaN()
}

// jint 数字取整（JS Number 后转 int）。
func jint(v any) int { return int(jnum(v)) }

// jintPtr 指针数字取整（nil → 0）。
func jintPtr(p *float64) int { return int(jnumPtr(p)) }

// finite 对应 Number.isFinite。
func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// jtruthy 对应 JS 真值判断（nil / "" / 0 / false 为假）。
func jtruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case bool:
		return x
	}
	return true
}

// deref 指针字符串取值（nil → ""）。
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// derefOr 指针字符串取值，nil 回落 fallback（对应 ??）。
func derefOr(p *string, fallback string) string {
	if p == nil {
		return fallback
	}
	return *p
}

// ptrOrNil 空串 → nil（JSON omitempty 语义）。
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
