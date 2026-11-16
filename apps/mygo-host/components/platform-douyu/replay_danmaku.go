// douyu VOD 弹幕:getBarrageList 分页拉全量(单页上限 500,游标 end_time,
// -1 即完);订阅时后台串行拉取,取消即停。col 为颜色索引(实测 0、7-13)。

package douyu

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

// vodColors VOD 样本实测的 7-13 色索引;1-6 未在 VOD 出现,复用直播 6 色兜底。
var vodColors = map[int]string{
	7:  "#FF5654",
	8:  "#FF7523",
	9:  "#FE69B3",
	10: "#FFBC00",
	11: "#78C946",
	12: "#9E7FFF",
	13: "#3D9BFF",
}

// vodColorOf 0 = 默认白(省略);未命中索引省略。
func vodColorOf(n float64) *string {
	i := int(n)
	if i <= 0 {
		return nil
	}
	if hex, ok := vodColors[i]; ok {
		return platform.Str(hex)
	}
	if hex, ok := douyuColors[i]; ok {
		return platform.Str(hex)
	}
	return nil
}

// mapBarrage 弹幕条目 → DanmakuItem(tl 为相对起点毫秒)。
func mapBarrage(list []map[string]any) []platform.DanmakuItem {
	items := make([]platform.DanmakuItem, 0, len(list))
	for _, row := range list {
		text := jsString(row["ctt"])
		if text == "" {
			continue
		}
		item := platform.DanmakuItem{Text: text, User: platform.Str(jsString(row["nn"]))}
		if tl, ok := toInt(row["tl"]); ok {
			ms := int64(tl)
			item.TimeMs = &ms
		}
		if col, ok := toInt(row["col"]); ok {
			item.Color = vodColorOf(col)
		}
		items = append(items, item)
	}
	return items
}

// vodDanmakuStream 订阅即后台串行分页拉取全量弹幕(逐页推送);退订停止。
func vodDanmakuStream(sc *platform.ServiceContext, hashID string) platform.DanmakuStream {
	return func(onItems func([]platform.DanmakuItem)) func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			start := int64(0)
			for attempts := 0; ; attempts++ {
				if ctx.Err() != nil {
					return
				}
				rows, endTime, err := fetchBarragePage(ctx, sc, hashID, start)
				if err != nil {
					sc.Log("douyu: VOD 弹幕第 %d 页失败: %v", attempts+1, err)
					return
				}
				if items := mapBarrage(rows); len(items) > 0 {
					onItems(items)
				}
				if endTime < 0 || endTime <= start {
					return // end_time=-1 即完;游标不前进防死循环
				}
				start = endTime
			}
		}()
		return cancel
	}
}

// fetchBarragePage 单页弹幕(500 条上限);end_time 为下一页游标。
func fetchBarragePage(ctx context.Context, sc *platform.ServiceContext, hashID string, startMs int64) ([]map[string]any, int64, error) {
	url := fmt.Sprintf("%s/wgapi/vod/center/getBarrageList?vid=%s&start_time=%d&end_time=-1", vodBaseURL, hashID, startMs)
	var res barrageResponse
	if err := vodGetJSON(ctx, sc, url, &res); err != nil {
		return nil, 0, err
	}
	if n, ok := toInt(res.Error); ok && n != 0 {
		return nil, 0, fmt.Errorf("error %s", jsString(res.Error))
	}
	end, _ := toInt(res.Data.EndTime)
	return res.Data.List, int64(end), nil
}
