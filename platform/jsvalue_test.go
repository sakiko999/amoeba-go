// JS 语义 helper 黄金用例(对齐 TS utils 的 toInt / strOr / String 行为)。

package platform

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
		got, ok := ToInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ToInt(%v) = %v/%v, want %v/%v", c.in, got, ok, c.want, c.ok)
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
		if got := JSString(c.in); got != c.want {
			t.Errorf("JSString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSTruthy(t *testing.T) {
	for _, v := range []any{nil, "", float64(0), false} {
		if JSTruthy(v) {
			t.Errorf("JSTruthy(%v) = true", v)
		}
	}
	for _, v := range []any{"0", float64(1), true, map[string]any{}} {
		if !JSTruthy(v) {
			t.Errorf("JSTruthy(%v) = false", v)
		}
	}
}

func TestStrOr(t *testing.T) {
	if got := StrOr(nil); got != nil {
		t.Errorf("StrOr(nil) = %v", *got)
	}
	if got := StrOr(""); got != nil {
		t.Errorf("StrOr(空串) = %v", *got)
	}
	if got := StrOr("x"); got == nil || *got != "x" {
		t.Errorf("StrOr(x) = %v", got)
	}
	if got := StrOr(float64(42)); got == nil || *got != "42" {
		t.Errorf("StrOr(42) = %v", got)
	}
}
