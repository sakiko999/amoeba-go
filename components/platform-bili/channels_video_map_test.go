// 视频类渠道映射测试（离线）：热门/入站必刷/排行/每周必看/UP 主投稿。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestPopularChannelMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
			{"title":"P1","aid":1,"bvid":"BV1xx411c7mD","pubdate":1700000000,"owner":{"name":"UP"}}
		],"no_more":false}}`), nil
	})
	ch := channelByKey(t, list, "bili:popular")
	result := fetchSource(t, ch, platform.SourceInfo{})
	if len(result.Items) != 1 || result.Items[0].ID != "BV1xx411c7mD" {
		t.Fatalf("items = %+v", result.Items)
	}
	page := fetchMorePage(t, ch, platform.SourceInfo{}, "2")
	mustContain(t, fs.find("x/web-interface/popular?pn=2&ps=20").URL, "pn=2", "第二页参数")
	if page.HasMore == nil || !*page.HasMore || page.Cursor == nil || *page.Cursor != "3" {
		t.Errorf("hasMore/cursor = %v/%v", page.HasMore, page.Cursor)
	}
}

func TestPreciousChannelMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
			{"title":"必刷","aid":9,"bvid":"BV1xx411c7mD","pubdate":1600000000,"pic":"https://i0.hdslb.com/c.jpg"}
		]}}`), nil
	})
	ch := channelByKey(t, list, "bili:precious")
	result := fetchSource(t, ch, platform.SourceInfo{})
	item := result.Items[0]
	if item.PublishedAt != nil {
		t.Errorf("人工精选榜不应写 publishedAt: %v", item.PublishedAt)
	}
	if item.URL == nil || *item.URL != "https://www.bilibili.com/video/BV1xx411c7mD" {
		t.Errorf("forceBvid 链接 = %v", item.URL)
	}
	req := fs.find("/x/web-interface/popular/precious")
	if req.Header.Get("referer") != "https://www.bilibili.com/v/popular/history" {
		t.Errorf("referer = %q", req.Header.Get("referer"))
	}
}

func TestRankingChannelMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
			{"title":"榜单视频","aid":2,"bvid":"BV1xx411c7mD","ctime":1700000123,"desc":""}
		]}}`), nil
	})
	ch := channelByKey(t, list, "bili:ranking")
	result := fetchSource(t, ch, platform.SourceInfo{"rid": "douga"})
	item := result.Items[0]
	if item.Summary == nil || *item.Summary != "榜单视频" {
		t.Errorf("desc 空时应回落 title: %v", item.Summary)
	}
	if item.PublishedAt == nil || *item.PublishedAt != 1_700_000_123_000 {
		t.Errorf("publishedAt 应取 ctime: %v", item.PublishedAt)
	}
	req := fs.find("x/web-interface/ranking/v2")
	mustContain(t, req.URL, "rid=1005", "英文分区映射")
	if req.Header.Get("referer") != "https://www.bilibili.com/v/popular/rank/douga" {
		t.Errorf("referer = %q", req.Header.Get("referer"))
	}
	// 数字 rid 原样透传。
	if _, err := ch.GetSource(platform.SourceInfo{"rid": "1003"}).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	mustContain(t, fs.find("x/web-interface/ranking/v2").URL, "rid=1003", "数字 rid")
}

func TestWeeklyChannelMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		url := req.URL.String()
		if strings.Contains(url, "popular/selected/series") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":[{"number":55,"name":"旧"},{"number":62,"name":"新"}]}`), nil
		}
		mustContain(t, url, "number=62", "取期号最大一期")
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
			{"title":"T","bvid":"BV1xx411c7mD","cover":"https://i0.hdslb.com/w.jpg","param":7,"rcmd_reason":"推荐"}
		]}}`), nil
	})
	ch := channelByKey(t, list, "bili:weekly")
	result := fetchSource(t, ch, platform.SourceInfo{})
	item := result.Items[0]
	if item.ID != "BV1xx411c7mD" || item.PublishedAt != nil {
		t.Errorf("id/publishedAt = %q/%v", item.ID, item.PublishedAt)
	}
	if item.Summary == nil || *item.Summary != "新 T - 推荐" {
		t.Errorf("summary = %v", item.Summary)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://i0.hdslb.com/w.jpg" {
		t.Errorf("thumbnail = %v", item.Thumbnail)
	}
	selected := fs.find("popular/selected?type=weekly_selected")
	if selected == nil || !strings.Contains(selected.Header.Get("referer"), "navhide=1") {
		t.Errorf("每周必看 referer = %+v", selected)
	}
}

func TestWeeklyNoSeriesError(t *testing.T) {
	_, _, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":[]}`), nil
	})
	ch := channelByKey(t, list, "bili:weekly")
	_, err := ch.GetSource(platform.SourceInfo{}).Fetch(context.Background())
	if err == nil || err.Error() != "bilibili weekly: no series" {
		t.Fatalf("err = %v", err)
	}
}

func TestUserVideoMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{
			"list":{"vlist":[{"title":"投稿","aid":3,"bvid":"BV1xx411c7mD","created":1700000000,
				"duration":100,"description":"简介","author":"UP主","is_charging_arc":true}]},
			"page":{"count":45}}}`), nil
	})
	ch := channelByKey(t, list, "bili:user_video")
	page := fetchMorePage(t, ch, platform.SourceInfo{"uid": "42", "cookie": "SESSDATA=abc"}, "2")
	if len(page.Items) != 1 {
		t.Fatalf("items = %d", len(page.Items))
	}
	item := page.Items[0]
	if item.MembersOnly == nil || !*item.MembersOnly {
		t.Error("is_charging_arc 应标 membersOnly")
	}
	if item.Duration != nil {
		t.Errorf("space 投稿 TS 不传 duration: %v", *item.Duration)
	}
	if item.Author == nil || item.Author.Name != "UP主" || item.Summary == nil || *item.Summary != "简介" {
		t.Errorf("author/summary = %+v/%v", item.Author, item.Summary)
	}
	if page.HasMore == nil || *page.HasMore || page.Total == nil || *page.Total != 45 {
		t.Errorf("hasMore/total = %v/%v（2*30<45 应为 false）", page.HasMore, page.Total)
	}
	req := fs.find("x/space/wbi/arc/search")
	mustContain(t, req.URL, "mid=42", "mid 参数")
	mustContain(t, req.URL, "dm_img_list=", "space 指纹")
	mustContain(t, req.Header.Get("cookie"), "SESSDATA=abc", "登录 cookie")
	mustContain(t, req.Header.Get("cookie"), "buvid3=ANON_B3", "buvid 兜底")
	if req.Header.Get("referer") != "https://space.bilibili.com/42/video" {
		t.Errorf("referer = %q", req.Header.Get("referer"))
	}
	if _, err := ch.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil ||
		err.Error() != "bili:user_video 需要 uid" {
		t.Errorf("缺 uid err = %v", err)
	}
}

func TestPopularNoMore(t *testing.T) {
	_, _, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[{"title":"x","aid":1}],"no_more":true}}`), nil
	})
	ch := channelByKey(t, list, "bili:popular")
	page := fetchMorePage(t, ch, platform.SourceInfo{}, "2")
	if page.HasMore == nil || *page.HasMore || page.Cursor != nil {
		t.Errorf("no_more=true 应无 cursor: %v/%v", page.HasMore, page.Cursor)
	}
	if _, err := ch.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
}
