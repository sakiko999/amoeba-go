// bili 搜索端点：直播房间 / 主播（live_user）/ UP 主（bili_user）。
// 搜索匿名可用，标题带 <em> 高亮需剥离；/search/type 统一走 searchType。

package bili

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// searchBiliRooms 搜索直播房间（匿名可用；pageSize 钳到 1..50）。
func (c *client) searchBiliRooms(ctx context.Context, keyword string, page, pageSize int) ([]platform.Item, error) {
	size := pageSize
	if size < 1 {
		size = 1
	}
	if size > 50 {
		size = 50
	}
	params := encodePairs([][2]string{
		{"search_type", "live"}, {"keyword", keyword},
		{"page", strconv.Itoa(page)}, {"page_size", strconv.Itoa(size)},
	})
	var resp struct {
		Data *struct {
			Result *struct {
				LiveRoom []map[string]any `json:"live_room"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := c.searchType(ctx, params, &resp); err != nil {
		return nil, err
	}
	t := c.now()
	items := []platform.Item{}
	if resp.Data == nil || resp.Data.Result == nil {
		return items, nil
	}
	for _, item := range resp.Data.Result.LiveRoom {
		roomID := jstr(item["roomid"])
		if roomID == "" {
			continue
		}
		it := platform.Item{
			ID:         "bilibili:" + roomID,
			SourceID:   "bili:search",
			Kind:       platform.KindLive,
			Title:      stripEm(jstr(item["title"])),
			URL:        platform.Str(liveBase + "/" + roomID),
			FetchedAt:  t,
			Platform:   "bilibili",
			RoomID:     platform.Str(roomID),
			LiveStatus: liveStatusPtr(jnum(item["live_status"]) == 1),
			Online:     platform.Int(jint(item["online"])),
		}
		if thumb := coverURL(item["cover"]); thumb != nil {
			it.Thumbnail = thumb
		} else if thumb := coverURL(item["user_cover"]); thumb != nil {
			it.Thumbnail = thumb
		}
		if uname := jstr(item["uname"]); uname != "" {
			it.Author = &platform.Author{Name: stripEm(uname)}
		}
		items = append(items, it)
	}
	return items, nil
}

// searchBiliAnchors 搜索主播（search_type=live_user；该接口无 page_size）。
func (c *client) searchBiliAnchors(ctx context.Context, keyword string, page int) ([]platform.Item, error) {
	params := encodePairs([][2]string{
		{"search_type", "live_user"}, {"keyword", keyword}, {"page", strconv.Itoa(page)},
	})
	var resp struct {
		Data *struct {
			Result []map[string]any `json:"result"`
		} `json:"data"`
	}
	if err := c.searchType(ctx, params, &resp); err != nil {
		return nil, err
	}
	t := c.now()
	items := []platform.Item{}
	if resp.Data == nil {
		return items, nil
	}
	for _, item := range resp.Data.Result {
		roomID := jstr(item["roomid"])
		if roomID == "" {
			continue
		}
		uname := jstr(item["uname"])
		it := platform.Item{
			ID:         "bilibili:" + roomID,
			SourceID:   "bili:anchor",
			Kind:       platform.KindLive,
			Title:      stripEm(uname),
			URL:        platform.Str(liveBase + "/" + roomID),
			FetchedAt:  t,
			Platform:   "bilibili",
			RoomID:     platform.Str(roomID),
			LiveStatus: liveStatusPtr(jnum(item["is_live"]) == 1),
		}
		it.Thumbnail = coverURL(item["uface"])
		author := &platform.Author{Name: stripEm(uname)}
		if uid := jstr(item["uid"]); uid != "" {
			author.Handle = platform.Str(uid)
		}
		it.Author = author
		items = append(items, it)
	}
	return items, nil
}

// biliUserHit 搜 UP 主命中（mid = bili:user_video 的 uid 参数）。
type biliUserHit struct {
	Mid    string
	Uname  string
	Fans   *int
	Videos *int
	Upic   *string
}

// searchBiliUsers 搜索视频 UP 主（search_type=bili_user，匿名 + buvid）。
func (c *client) searchBiliUsers(ctx context.Context, keyword string, page int) ([]biliUserHit, error) {
	params := encodePairs([][2]string{
		{"search_type", "bili_user"}, {"keyword", keyword}, {"page", strconv.Itoa(page)},
	})
	var resp struct {
		Data *struct {
			Result []map[string]any `json:"result"`
		} `json:"data"`
	}
	if err := c.searchType(ctx, params, &resp); err != nil {
		return nil, err
	}
	hits := []biliUserHit{}
	if resp.Data == nil {
		return hits, nil
	}
	for _, item := range resp.Data.Result {
		mid := jstr(item["mid"])
		if mid == "" {
			continue
		}
		hit := biliUserHit{Mid: mid, Uname: stripEm(jstr(item["uname"]))}
		if fans := jnum(item["fans"]); fans != 0 && finite(fans) {
			hit.Fans = platform.Int(int(fans))
		}
		if videos := jnum(item["videos"]); videos != 0 && finite(videos) {
			hit.Videos = platform.Int(int(videos))
		}
		hit.Upic = httpsPrefix(jstr(item["upic"]))
		hits = append(hits, hit)
	}
	return hits, nil
}

// searchType 搜索端点共用请求（referer 走 search 站，匿名 + buvid）。
func (c *client) searchType(ctx context.Context, params string, out any) error {
	url := apiMain + "/x/web-interface/search/type?" + params
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: "https://search.bilibili.com/", Buvid: true})
	if err != nil {
		return err
	}
	return platform.UnmarshalJSON(raw, out)
}

var emTagRe = regexp.MustCompile(`</?em\b[^>]*>`)

// stripEm 剥离搜索标题里的 <em class="keyword"> 高亮标签。
func stripEm(raw string) string { return emTagRe.ReplaceAllString(raw, "") }

// coverURL 封面补尺寸后缀（// 开头补 https:；已带 @ 参数不重复加）。
func coverURL(raw any) *string {
	s := jstr(raw)
	if s == "" {
		return nil
	}
	full := s
	if strings.HasPrefix(s, "//") {
		full = "https:" + s
	}
	if strings.Contains(full, "@") {
		return &full
	}
	full += "@400w.jpg"
	return &full
}

// httpsPrefix //host/… → https://host/…（空串 → nil）。
func httpsPrefix(raw string) *string {
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	return &raw
}
