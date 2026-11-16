// bilibili 视频弹幕：view 拿 cid+duration → 逐段 seg.so（并行）→ protobuf 解码。
// 每段 6 分钟，段数 = ceil(durationSec/360) + 1；订阅时一次性 load 全量推给 onItems。

package bili

import (
	"context"
	"fmt"
	"math"
	"sync"

	"mygo-host/platform"
)

const segmentSec = 360

// fetchDanmakuSeg 抓一段 seg.so；非 200 / 空段返回 nil（越界段天然跳过）。
func (c *client) fetchDanmakuSeg(ctx context.Context, cid, n int) []byte {
	url := fmt.Sprintf("%s/x/v2/dm/web/seg.so?type=1&oid=%d&segment_index=%d", apiMain, cid, n)
	res, err := c.sc.HTTP.Do(ctx, platform.Request{
		URL:     url,
		Headers: map[string]string{"user-agent": bilibiliUA, "referer": "https://www.bilibili.com/video/"},
	})
	if err != nil || res.Status != 200 || res.Body == "" {
		return nil
	}
	return []byte(res.Body)
}

// resolveBiliDanmaku bvid/aid → view(cid+duration) → 逐段 seg.so → DanmakuItem[]。
func (c *client) resolveBiliDanmaku(ctx context.Context, itemID string, cookie *string) ([]platform.DanmakuItem, error) {
	url := apiMain + "/x/web-interface/view?" + viewIDParam(itemID)
	referer := "https://www.bilibili.com/video/" + itemID
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: referer, Cookie: cookie})
	if err != nil {
		return nil, err
	}
	var view viewResp
	if err := platform.UnmarshalJSON(raw, &view); err != nil {
		return nil, err
	}
	if view.Data == nil || view.Data.Cid == nil || *view.Data.Cid == 0 {
		return nil, fmt.Errorf("bilibili danmaku: no cid for %s", itemID)
	}
	cid := int(*view.Data.Cid)
	duration := jnumPtr(view.Data.Duration)
	segments := int(math.Ceil(duration/segmentSec)) + 1
	if segments < 1 {
		segments = 1
	}
	segs := make([][]byte, segments)
	var wg sync.WaitGroup
	for i := 0; i < segments; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			segs[idx] = c.fetchDanmakuSeg(ctx, cid, idx+1)
		}(i)
	}
	wg.Wait()

	items := []platform.DanmakuItem{}
	for _, buf := range segs {
		for _, e := range decodeDanmakuSeg(buf) {
			items = append(items, platform.DanmakuItem{
				Text:   e.content,
				Mode:   platform.Int(e.mode),
				TimeMs: platform.Int64(int64(e.progress)),
				Color:  platform.Str(argbToHex(e.color)),
			})
		}
	}
	return items, nil
}

// vodDanmakuStream 订阅时一次性解析全量并推送（首轮失败只记日志）。
// cookie 三态(nil=默认解析 / 空串=强制匿名 / 非空=显式)透传给解析层。
func (c *client) vodDanmakuStream(itemID string, cookie *string) platform.DanmakuStream {
	return platform.DeferredStream(
		func() ([]platform.DanmakuItem, error) {
			return c.resolveBiliDanmaku(context.Background(), itemID, cookie)
		},
		func(items []platform.DanmakuItem, onItems func([]platform.DanmakuItem)) func() {
			if len(items) > 0 {
				onItems(items)
			}
			return nil
		},
		func(err error) { c.log("bili: 视频弹幕懒解析失败: %v", err) },
	)
}
