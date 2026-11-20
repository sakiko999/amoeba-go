// douyin 通用辅助:encodeURIComponent / URLSearchParams 编码、末 N 字符与
// 粉丝数缩写;全部对齐 TS 原文语义,不引入第三方依赖。

package douyin

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"mygo-host/platform"
)

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

// formEncode 等价 URLSearchParams 序列化(空格 → +,保留 * - . _)。
func formEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '*' || c == '-' || c == '.' || c == '_':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

// lastN 取末 n 个 rune(TS slice(-n);按 rune 避免截断代理对)。
func lastN(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := utf8.RuneCountInString(s)
	if count <= n {
		return s
	}
	start := count - n
	index := 0
	for pos := range s {
		if index == start {
			return s[pos:]
		}
		index++
	}
	return s
}

// formatCount 粉丝数缩写:1.2万 / 1.3亿(TS fmtCount,保留 1 位小数去 .0)。
func formatCount(n float64) string {
	switch {
	case n >= 1e8:
		return fixed1(n/1e8) + "亿"
	case n >= 1e4:
		return fixed1(n/1e4) + "万"
	default:
		return platform.JSString(n)
	}
}

func fixed1(v float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0")
}

// truncate 截断到 n 字节(签名 URL 为 ASCII;TS slice(0,80))。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
