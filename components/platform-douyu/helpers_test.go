// douyu 特有 helper 的黄金用例(parseCount 的「万/w/k」归一,对齐 TS parseCount)。

package douyu

import "testing"

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
