// JS 语义 helper 的黄金用例(对齐 TS utils 的 toInt / parseCount / String 行为)。

package douyu

import (
	"math"
	"testing"
)

func TestToInt(t *testing.T) {
	cases := []struct {
		in   any
		want float64
		ok   bool
	}{
		{nil, 0, false},
		{"", 0, false},
		{"1", 1, true},
		{"1.5", 1.5, true},
		{float64(0), 0, true},
		{true, 1, true},
		{false, 0, true},
		{"abc", 0, false},
		{" 12 ", 12, true},
		{math.Inf(1), 0, false},
	}
	for _, c := range cases {
		got, ok := toInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("toInt(%v) = %v/%v, want %v/%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseCount(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{nil, 0, false},
		{float64(1234), 1234, true},
		{float64(0), 0, true},
		{"133.1万", 1331000, true},
		{"1.2w", 12000, true},
		{"3.4k", 3400, true},
		{"10万+", 100000, true},
		{"999", 999, true},
		{".5万", 5000, true},
		{"1.2.3万", 12000, true},
		{"abc", 0, false},
		{"", 0, false},
		{true, 0, false},
	}
	for _, c := range cases {
		got := parseCount(c.in)
		if (got != nil) != c.ok || (got != nil && *got != c.want) {
			t.Errorf("parseCount(%v) = %v, want %d/%v", c.in, got, c.want, c.ok)
		}
	}
}

func TestJSStringNumbers(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{float64(9527), "9527"},
		{float64(1000000), "1000000"},
		{float64(1.5), "1.5"},
		{float64(0), "0"},
		{nil, ""},
		{true, "true"},
		{"x", "x"},
	}
	for _, c := range cases {
		if got := jsString(c.in); got != c.want {
			t.Errorf("jsString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTruthy(t *testing.T) {
	for _, v := range []any{nil, "", float64(0), false} {
		if truthy(v) {
			t.Errorf("truthy(%v) = true", v)
		}
	}
	for _, v := range []any{"0", float64(1), true, map[string]any{}} {
		if !truthy(v) {
			t.Errorf("truthy(%v) = false", v)
		}
	}
}
