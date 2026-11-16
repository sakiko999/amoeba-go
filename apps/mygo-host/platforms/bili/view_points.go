// UP 主在投稿里标的章节时间点（/x/player/v2 的 view_points，非 wbi 接口）。
// 多数视频没有 → 空数组；调用方按「没有章节」处理。

package bili

import (
	"context"
	"math"
	"sort"
	"strings"

	"mygo-host/platform"
)

func (c *client) viewPoints(ctx context.Context, id, cid string, cookie *string) ([]platform.PlaybackChapter, error) {
	url := apiMain + "/x/player/v2?" + viewIDParam(id) + "&cid=" + cid
	referer := "https://www.bilibili.com/video/" + id
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: referer, Cookie: cookie})
	if err != nil {
		return nil, err
	}
	var resp playerV2Resp
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return nil, err
	}
	chapters := []platform.PlaybackChapter{}
	if resp.Data == nil {
		return chapters, nil
	}
	for _, p := range resp.Data.ViewPoints {
		t := math.Round(jnumPtr(p.From))
		if !finite(t) || t < 0 {
			t = 0
		}
		label := strings.TrimSpace(deref(p.Content))
		if label == "" {
			continue
		}
		chapters = append(chapters, platform.PlaybackChapter{T: t, Label: label})
	}
	sort.SliceStable(chapters, func(i, j int) bool { return chapters[i].T < chapters[j].T })
	return chapters, nil
}
