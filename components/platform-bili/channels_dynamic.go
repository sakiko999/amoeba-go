// bili:dynamic —— B站用户动态（social，offset 字符串游标翻页）。
// 端点 feed/space 需 Referer(space/{uid}/) + 登录 Cookie(SESSDATA)，未登录 -101。
// 动态类型：转发(desc+orig) / 图文(opus pics+summary) / 视频(archive) / 专栏(opus cv 号)。
// offset 游标不是数字，platform.MakeChannel 的数值翻页不适用，故自定义 DataSource。
// 结构定义与正文解析见 dynamic_parse.go。

package bili

import (
	"context"
	"fmt"
	"math"
	"sync"

	"mygo-host/platform"
)

type dynamicChannel struct {
	c *client
}

func (d *dynamicChannel) Key() string         { return "bili:dynamic" }
func (d *dynamicChannel) Name() string        { return "B站用户动态" }
func (d *dynamicChannel) Kind() platform.Kind { return platform.KindSocial }
func (d *dynamicChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{requiredField("uid", "UP 主 uid")}
}
func (d *dynamicChannel) DefaultInfo() platform.SourceInfo { return nil }

func (d *dynamicChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &dynamicSource{c: d.c, info: info}
}

type dynamicSource struct {
	c    *client
	info platform.SourceInfo
}

// Fetch 首页（不带 offset）。
func (s *dynamicSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	uid, err := requireDynUID(s.info)
	if err != nil {
		return platform.FetchResult{}, err
	}
	page, err := s.c.fetchDynamicRaw(ctx, uid, "", nonEmptyCookie(s.info))
	if err != nil {
		return platform.FetchResult{}, err
	}
	return platform.FetchResult{Items: s.c.dynamicItems(ctx, page.items, s.c.now())}, nil
}

// FetchMore offset 游标翻页：无 cursor 时先拉首页拿下一页 token（条目丢弃）。
func (s *dynamicSource) FetchMore(ctx context.Context, cursor string, existing int) (platform.FetchPage, error) {
	uid, err := requireDynUID(s.info)
	if err != nil {
		return platform.FetchPage{}, err
	}
	token := cursor
	if token == "" {
		first, err := s.c.fetchDynamicRaw(ctx, uid, "", nonEmptyCookie(s.info))
		if err != nil {
			return platform.FetchPage{}, err
		}
		token = first.offset
		if token == "" {
			return platform.FetchPage{Items: []platform.Item{}}, nil
		}
	}
	page, err := s.c.fetchDynamicRaw(ctx, uid, token, nonEmptyCookie(s.info))
	if err != nil {
		return platform.FetchPage{}, err
	}
	result := platform.FetchPage{Items: s.c.dynamicItems(ctx, page.items, s.c.now())}
	// has_more=true 但 offset 缺失时不给 cursor（消费方判到底，不悬挂重试）。
	if page.hasMore && page.offset != "" {
		next := page.offset
		result.Cursor = &next
		result.HasMore = platform.Bool(true)
	}
	return result, nil
}

func requireDynUID(info platform.SourceInfo) (string, error) {
	uid := info["uid"]
	if !digitsRe.MatchString(uid) {
		return "", fmt.Errorf("bili:dynamic 需要数字 uid,收到 %q", uid)
	}
	return uid, nil
}

type dynPage struct {
	items   []dynItem
	hasMore bool
	offset  string
}

// fetchDynamicRaw 拉一页原始动态（space 动态端点要登录 cookie + wbi 域外 referer）。
func (c *client) fetchDynamicRaw(ctx context.Context, uid, offset string, cookie *string) (dynPage, error) {
	url := fmt.Sprintf("%s/x/polymer/web-dynamic/v1/feed/space?host_mid=%s&platform=web&features=itemOpusStyle,listOnlyfans,opusBigCover,onlyfansVote", apiMain, uid)
	if offset != "" {
		url += "&offset=" + encodeURIComponent(offset)
	}
	raw, _, err := c.getJSON(ctx, url, reqOptions{
		Referer: "https://space.bilibili.com/" + uid + "/",
		Cookie:  cookie,
	})
	if err != nil {
		return dynPage{}, err
	}
	var resp struct {
		Data *struct {
			Items   []dynItem `json:"items"`
			HasMore *bool     `json:"has_more"`
			Offset  *string   `json:"offset"`
		} `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return dynPage{}, err
	}
	page := dynPage{}
	if resp.Data != nil {
		page.items = resp.Data.Items
		page.hasMore = resp.Data.HasMore != nil && *resp.Data.HasMore
		page.offset = deref(resp.Data.Offset)
	}
	return page, nil
}

// dynamicItems DynItem[] → Social[]（先 parse 一次，补全缺宽高后再组装）。
func (c *client) dynamicItems(ctx context.Context, items []dynItem, t int64) []platform.Item {
	parsed := make([]parsedDyn, len(items))
	var wg sync.WaitGroup
	for i := range items {
		parsed[i] = parseDynItem(&items[i])
		if len(parsed[i].images) == 0 {
			continue
		}
		wg.Add(1)
		go func(imgs []platform.SocialImage) {
			defer wg.Done()
			fillImageSizes(ctx, c.sc.HTTP, imgs)
		}(parsed[i].images)
	}
	wg.Wait()

	out := make([]platform.Item, 0, len(items))
	for i := range items {
		idStr := deref(items[i].IDStr)
		content := parsed[i].content
		title := parsed[i].title
		if title == "" {
			title = jsSlice(content, 30)
		}
		item := platform.Item{
			ID:        "bili-dyn-" + idStr,
			SourceID:  "bili:dynamic",
			Kind:      platform.KindSocial,
			Title:     title,
			URL:       platform.Str("https://t.bilibili.com/" + idStr),
			Content:   platform.Str(content),
			FetchedAt: t,
		}
		if len(parsed[i].images) > 0 {
			item.Images = parsed[i].images
		}
		if parsed[i].likes != 0 {
			item.Likes = platform.Int(parsed[i].likes)
		}
		var author *dynModuleAuthor
		if items[i].Modules != nil {
			author = items[i].Modules.ModuleAuthor
		}
		if author != nil && deref(author.Name) != "" {
			item.Author = &platform.Author{Name: deref(author.Name), Avatar: author.Face}
		}
		if author != nil && author.PubTs != nil && !math.IsNaN(*author.PubTs) && *author.PubTs != 0 {
			item.PublishedAt = platform.Int64(int64(*author.PubTs * 1000))
		}
		out = append(out, item)
	}
	return out
}
