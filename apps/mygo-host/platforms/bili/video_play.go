// bilibili UGC 视频懒解析：bvid/aid → cid → 全档位可播直链 + 章节。
// 章节接口要 cid，放在 playurl 之后且失败不拖垮播放（TS 同为 catch 空数组）。

package bili

import (
	"context"
	"fmt"

	"mygo-host/platform"
)

func (c *client) resolveVideoPlay(ctx context.Context, id string, info platform.ResolveInfo) (*platform.VideoPlayResult, error) {
	cookie := nonEmptyResolveCookie(info)
	cid, err := c.resolveCid(ctx, id, cookie)
	if err != nil {
		return nil, err
	}
	streams, err := c.resolvePlayURL(ctx, id, cid, cookie)
	if err != nil {
		return nil, err
	}
	chapters, err := c.viewPoints(ctx, id, cid, cookie)
	if err != nil {
		// TS catch 空数组:章节失败仍恒输出 chapters(空数组),与 op 结果形状一致。
		chapters = []platform.PlaybackChapter{}
	}
	return &platform.VideoPlayResult{Streams: streams, Chapters: chapters}, nil
}

// resolveCid bvid/aid → 默认分 P 的 cid（/x/web-interface/view，非 wbi）。
func (c *client) resolveCid(ctx context.Context, id string, cookie *string) (string, error) {
	url := apiMain + "/x/web-interface/view?" + viewIDParam(id)
	referer := "https://www.bilibili.com/video/" + id
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: referer, Cookie: cookie})
	if err != nil {
		return "", err
	}
	var resp viewResp
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return "", err
	}
	if resp.Data == nil || resp.Data.Cid == nil || *resp.Data.Cid == 0 {
		return "", fmt.Errorf("bilibili view: no cid for %s", id)
	}
	return jsNum(*resp.Data.Cid), nil
}

// viewIDParam view / player-v2 / 弹幕接口的 av 号用 aid=，BV 号走 bvid=。
func viewIDParam(id string) string {
	if len(id) > 2 && id[:2] == "av" {
		return "aid=" + encodeURIComponent(id[2:])
	}
	return "bvid=" + encodeURIComponent(id)
}

// playurlIDParam playurl 的 av 号只认 avid=（aid= / bvid=av123 均判非法）。
func playurlIDParam(id string) string {
	if len(id) > 2 && id[:2] == "av" {
		return "avid=" + encodeURIComponent(id[2:])
	}
	return "bvid=" + encodeURIComponent(id)
}
