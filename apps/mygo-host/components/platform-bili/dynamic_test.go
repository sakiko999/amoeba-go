// bili:dynamic 映射与 offset 游标测试（离线）：图文/转发拼接、宽高/点赞/作者时间，
// 以及「无 cursor 先拉首页换 token」的两步翻页语义。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

const dynPageJSON = `{"code":0,"data":{"items":[
	{"id_str":"1001","type":"DYNAMIC_TYPE_DRAW","modules":{
		"module_author":{"mid":42,"name":"UP","face":"https://i0.hdslb.com/f.jpg","pub_ts":1700000000},
		"module_dynamic":{"desc":{"text":"正文"},"major":{"type":"MAJOR_TYPE_OPUS","opus":{
			"summary":{"text":"  摘要  "},"pics":[{"url":"http://i0.hdslb.com/p.jpg","width":100,"height":200}],
			"jump_url":"https://www.bilibili.com/opus/123"}}},
		"module_stat":{"like":{"count":9}}}},
	{"id_str":"1002","type":"DYNAMIC_TYPE_FORWARD","modules":{
		"module_author":{"mid":7,"name":"转发者","pub_ts":0},
		"module_dynamic":{"desc":{"text":"转发"}},
		"module_stat":{"like":{"count":0}}},
	 "orig":{"id_str":"999","modules":{
		"module_author":{"name":"原作者"},
		"module_dynamic":{"major":{"type":"MAJOR_TYPE_ARCHIVE","archive":{
			"title":"视频标题","desc":"视频简介"}}}}}}
],"has_more":true,"offset":"OFF1"}}`

func TestDynamicItemsMapping(t *testing.T) {
	fs := &fakeServer{}
	fs.respond = func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, dynPageJSON), nil
	}
	c := newClient(newTestContext(t, fs, testNow))
	page, err := c.fetchDynamicRaw(context.Background(), "42", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	items := c.dynamicItems(context.Background(), page.items, testNow)
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	draw := items[0]
	if draw.ID != "bili-dyn-1001" || draw.URL == nil || *draw.URL != "https://t.bilibili.com/1001" {
		t.Errorf("id/url = %q/%v", draw.ID, draw.URL)
	}
	if draw.Kind != platform.KindSocial || draw.SourceID != "bili:dynamic" || draw.FetchedAt != testNow {
		t.Errorf("kind/sourceId/fetchedAt = %v/%v/%d", draw.Kind, draw.SourceID, draw.FetchedAt)
	}
	if draw.Content == nil || *draw.Content != "正文\n  摘要" || draw.Title != "正文\n  摘要" {
		t.Errorf("content/title = %v/%q（只 trim 整体，不 trim 摘要内部空白）", draw.Content, draw.Title)
	}
	if len(draw.Images) != 1 || draw.Images[0].URL != "https://i0.hdslb.com/p.jpg" ||
		draw.Images[0].Width == nil || *draw.Images[0].Width != 100 ||
		draw.Images[0].Height == nil || *draw.Images[0].Height != 200 {
		t.Errorf("images = %+v", draw.Images)
	}
	if draw.Likes == nil || *draw.Likes != 9 {
		t.Errorf("likes = %v", draw.Likes)
	}
	if draw.Author == nil || draw.Author.Name != "UP" || draw.Author.Avatar == nil || *draw.Author.Avatar != "https://i0.hdslb.com/f.jpg" {
		t.Errorf("author = %+v", draw.Author)
	}
	if draw.PublishedAt == nil || *draw.PublishedAt != 1_700_000_000_000 {
		t.Errorf("publishedAt = %v", draw.PublishedAt)
	}

	forward := items[1]
	want := "转发\n//转发自: @原作者:\n视频标题\n视频简介"
	if forward.Content == nil || *forward.Content != want {
		t.Errorf("forward content = %v, want %q", forward.Content, want)
	}
	if forward.Title != want {
		t.Errorf("forward title = %q（无 cv 号时取正文前 30 字符）", forward.Title)
	}
	if forward.Likes != nil {
		t.Errorf("likes=0 不应出字段: %v", forward.Likes)
	}
	if forward.PublishedAt != nil {
		t.Errorf("pub_ts=0 不应出 publishedAt: %v", forward.PublishedAt)
	}
	req := fs.find("feed/space")
	mustContain(t, req.URL, "host_mid=42", "uid 参数")
	if req.Header.Get("referer") != "https://space.bilibili.com/42/" {
		t.Errorf("referer = %q", req.Header.Get("referer"))
	}
}

func TestDynamicFetchMoreOffsetCursor(t *testing.T) {
	fs := &fakeServer{}
	fs.respond = func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, dynPageJSON), nil
	}
	c := newClient(newTestContext(t, fs, testNow))
	ch := channelByKey(t, channels(c), "bili:dynamic")
	source := ch.GetSource(platform.SourceInfo{"uid": "42", "cookie": "SESSDATA=abc"})
	pageable := source.(platform.PageableSource)

	page, err := pageable.FetchMore(context.Background(), "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Errorf("items = %d", len(page.Items))
	}
	if page.Cursor == nil || *page.Cursor != "OFF1" || page.HasMore == nil || !*page.HasMore {
		t.Errorf("cursor/hasMore = %v/%v", page.Cursor, page.HasMore)
	}
	if len(fs.requests) != 2 {
		t.Fatalf("无 cursor 应先拉首页换 token: requests = %d", len(fs.requests))
	}
	if strings.Contains(fs.requests[0].URL, "offset=") {
		t.Errorf("首页请求不应带 offset: %s", fs.requests[0].URL)
	}
	mustContain(t, fs.requests[1].URL, "offset=OFF1", "第二步 offset")
	if fs.requests[1].Header.Get("cookie") != "SESSDATA=abc" {
		t.Errorf("cookie 未透传: %q", fs.requests[1].Header.Get("cookie"))
	}

	// 错误 uid 在发请求前就报错。
	if _, err := ch.GetSource(platform.SourceInfo{"uid": "abc"}).Fetch(context.Background()); err == nil ||
		!strings.HasPrefix(err.Error(), "bili:dynamic 需要数字 uid") {
		t.Errorf("非法 uid err = %v", err)
	}
}

func TestDynamicMissingOffsetStops(t *testing.T) {
	fs := &fakeServer{}
	fs.respond = func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"items":[{"id_str":"1"}],"has_more":true}}`), nil
	}
	c := newClient(newTestContext(t, fs, testNow))
	ch := channelByKey(t, channels(c), "bili:dynamic")
	page, err := ch.GetSource(platform.SourceInfo{"uid": "42"}).(platform.PageableSource).
		FetchMore(context.Background(), "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.Cursor != nil || page.HasMore != nil {
		t.Errorf("offset 缺失应返回空页且不悬挂 cursor: %+v", page)
	}
	if len(fs.requests) != 1 {
		t.Errorf("只应请求首页一次: %d", len(fs.requests))
	}
}
