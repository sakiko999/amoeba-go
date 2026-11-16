// 合集类与搜索渠道映射测试（离线）：收藏夹/合集/视频列表的分页与 memo，
// 搜主播/搜 UP 主的 <em> 剥离与候选字段。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestFavlistMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"medias":[
			{"title":"有效","bvid":"BV1xx411c7mD","cover":"https://i0.hdslb.com/f.jpg","intro":"介绍",
			 "pubtime":1700000000,"upper":{"name":"UP"},"attr":0},
			{"title":"失效","bvid":"BV1xx411c7mD","attr":1},
			{"title":"无 ID","attr":0}
		],"has_more":true}}`), nil
	})
	ch := channelByKey(t, list, "bili:favlist")
	page := fetchMorePage(t, ch, platform.SourceInfo{"mediaId": "1052622027"}, "2")
	if len(page.Items) != 1 {
		t.Fatalf("过滤后 items = %d", len(page.Items))
	}
	item := page.Items[0]
	if item.Thumbnail == nil || *item.Thumbnail != "https://i0.hdslb.com/f.jpg" ||
		item.Author == nil || item.Author.Name != "UP" || item.Summary == nil || *item.Summary != "介绍" {
		t.Errorf("映射 = %+v", item)
	}
	if item.URL == nil || !strings.Contains(*item.URL, "BV1xx411c7mD") {
		t.Errorf("forceBvid url = %v", item.URL)
	}
	if page.HasMore == nil || !*page.HasMore || page.Cursor == nil || *page.Cursor != "3" {
		t.Errorf("hasMore/cursor = %v/%v", page.HasMore, page.Cursor)
	}
	mustContain(t, fs.find("fav/resource/list").URL, "media_id=1052622027", "media_id")
	_, err := ch.GetSource(platform.SourceInfo{"mediaId": "xxx"}).Fetch(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "bili:favlist 需要收藏夹 ID 或链接") {
		t.Errorf("非法 mediaId err = %v", err)
	}
}

func TestSeasonChannelMapping(t *testing.T) {
	viewCalls := 0
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		url := req.URL.String()
		if strings.Contains(url, "x/web-interface/view") {
			viewCalls++
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"ugc_season":{"id":2974525}}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"archives":[
			{"title":"A","bvid":"BV1xx411c7mD","pubdate":1700000000,
			 "owner":{"name":"不该有"},"is_charging_arc":true},
			{"title":"B","aid":9,"desc":"不该出现"},
			{"title":"空"}
		]}}`), nil
	})
	ch := channelByKey(t, list, "bili:season")
	info := platform.SourceInfo{"seasonId": "BV1Tb421b7mi"}
	result := fetchSource(t, ch, info)
	if len(result.Items) != 2 || viewCalls != 1 {
		t.Fatalf("items/viewCalls = %d/%d", len(result.Items), viewCalls)
	}
	// TS season 不传 owner/charging（合集条目没有作者，is_charging_arc 也不读）。
	if result.Items[0].Author != nil || result.Items[0].MembersOnly != nil {
		t.Errorf("season 不应带 author/membersOnly = %+v", result.Items[0])
	}
	if result.Items[1].ID != "av9" || result.Items[1].Summary != nil {
		t.Errorf("aid-only 映射 = %+v", result.Items[1])
	}
	// 同一输入的第二次 fetch 必须命中 memo（不再请求 view）。
	if _, err := ch.GetSource(info).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if viewCalls != 1 {
		t.Errorf("season memo 失效: viewCalls = %d", viewCalls)
	}
	mustContain(t, fs.find("seasons_archives_list").URL, "sort_reverse=true", "倒序参数")
	if ref := fs.find("seasons_archives_list"); ref.Header.Get("referer") != "https://www.bilibili.com/" {
		t.Errorf("referer = %q", ref.Header.Get("referer"))
	}
	if _, err := ch.GetSource(platform.SourceInfo{"seasonId": "hello"}).Fetch(context.Background()); err == nil ||
		!strings.HasPrefix(err.Error(), "bili:season 需要合集 ID 或视频链接") {
		t.Errorf("非法输入 err = %v", err)
	}
}

func TestSeasonVideoWithoutSeason(t *testing.T) {
	_, _, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
	})
	_, err := channelByKey(t, list, "bili:season").
		GetSource(platform.SourceInfo{"seasonId": "BV1Tb421b7mi"}).Fetch(context.Background())
	if err == nil || err.Error() != "视频 BV1Tb421b7mi 不属于任何合集" {
		t.Fatalf("err = %v", err)
	}
}

func TestSeriesChannelMapping(t *testing.T) {
	metaCalls := 0
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/x/series/series?") {
			metaCalls++
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"meta":{"mid":1557073149}}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"archives":[
			{"title":"S","bvid":"BV1xx411c7mD","desc":"简介","pubdate":1700000000,
			 "owner":{"name":"不该有"},"is_charging_arc":true}
		]}}`), nil
	})
	ch := channelByKey(t, list, "bili:series")
	info := platform.SourceInfo{"seriesId": "358495"}
	result := fetchSource(t, ch, info)
	if len(result.Items) != 1 || metaCalls != 1 {
		t.Fatalf("items/metaCalls = %d/%d", len(result.Items), metaCalls)
	}
	if result.Items[0].Summary == nil || *result.Items[0].Summary != "简介" {
		t.Errorf("series summary = %v", result.Items[0].Summary)
	}
	if result.Items[0].Author != nil || result.Items[0].MembersOnly != nil {
		t.Errorf("series 不应带 author/membersOnly = %+v", result.Items[0])
	}
	if _, err := ch.GetSource(info).Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if metaCalls != 1 {
		t.Errorf("series memo 失效: metaCalls = %d", metaCalls)
	}
	mustContain(t, fs.find("x/series/archives").URL, "mid=1557073149", "mid 参数")
	mustContain(t, fs.find("x/series/archives").URL, "pn=1", "pn 参数")
	if _, err := ch.GetSource(platform.SourceInfo{"seriesId": "space.bilibili.com/1/lists/abc"}).Fetch(context.Background()); err == nil ||
		!strings.HasPrefix(err.Error(), "bili:series 需要视频列表 ID 或链接") {
		t.Errorf("非法输入 err = %v", err)
	}
}

func TestSearchChannelsMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "search_type=live_user") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"result":[
				{"roomid":6,"uname":"<em class=\"keyword\">主播</em>","uface":"//i0.hdslb.com/u.jpg","uid":42,"is_live":1}
			]}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"result":[
			{"mid":42,"uname":"<em>UP</em>","fans":12,"videos":3,"upic":"//i0.hdslb.com/up.jpg"},
			{"mid":0,"uname":"空"}
		]}}`), nil
	})

	anchor := channelByKey(t, list, "bili:anchor")
	page := fetchMorePage(t, anchor, platform.SourceInfo{"keyword": "主播"}, "2")
	item := page.Items[0]
	if item.Title != "主播" || item.Author == nil || item.Author.Name != "主播" || item.Author.Handle == nil || *item.Author.Handle != "42" {
		t.Errorf("anchor 映射 = %+v", item)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://i0.hdslb.com/u.jpg@400w.jpg" {
		t.Errorf("anchor thumbnail = %v", item.Thumbnail)
	}
	if item.LiveStatus == nil || *item.LiveStatus != "live" {
		t.Errorf("anchor liveStatus = %v", item.LiveStatus)
	}
	if strings.Contains(fs.find("search_type=live_user").URL, "page_size=") {
		t.Error("live_user 不应带 page_size")
	}
	if _, err := anchor.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil ||
		err.Error() != "bili:anchor 需要 keyword 参数" {
		t.Errorf("anchor 缺 keyword err = %v", err)
	}

	user := channelByKey(t, list, "bili:user")
	result := fetchSource(t, user, platform.SourceInfo{"keyword": "UP"})
	// mid=0 也是合法命中（TS String(0 ?? "") === "0"），故两条都保留。
	if len(result.Items) != 2 {
		t.Fatalf("user items = %d", len(result.Items))
	}
	hit := result.Items[0]
	if hit.ID != "up-42" || hit.Title != "UP" || hit.URL == nil || *hit.URL != "https://space.bilibili.com/42" {
		t.Errorf("user 映射 = %+v", hit)
	}
	if hit.Thumbnail == nil || *hit.Thumbnail != "https://i0.hdslb.com/up.jpg" {
		t.Errorf("用户头像只补 https:（不加 @400w）: %v", hit.Thumbnail)
	}
	if hit.Summary == nil || *hit.Summary != "12 粉丝 · 3 个视频" {
		t.Errorf("user summary = %v", hit.Summary)
	}
	if _, err := user.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil ||
		err.Error() != "bili:user 需要 keyword 参数" {
		t.Errorf("user 缺 keyword err = %v", err)
	}
}
