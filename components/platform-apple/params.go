// 渠道参数解析:对齐 TS 的 `Number(info.x ?? def) || def`、trim 与数值夹取语义。

package apple

import (
	"math"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// jsNumber 近似 JS Number(string):空白 → 0,无法解析 → NaN。
func jsNumber(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return math.NaN()
	}
	return value
}

// paramNumber 近似 `Number(info[key] ?? def) || def`:缺失 / 0 / 非数回退默认值。
func paramNumber(info platform.SourceInfo, key string, def int) int {
	raw, ok := info[key]
	if !ok {
		return def
	}
	value := jsNumber(raw)
	if math.IsNaN(value) || math.IsInf(value, 0) || value == 0 {
		return def
	}
	return int(value)
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// paramCountry 国家区:trim 后为空回退 cn。
func paramCountry(info platform.SourceInfo) string {
	if country := strings.TrimSpace(info["country"]); country != "" {
		return country
	}
	return "cn"
}
