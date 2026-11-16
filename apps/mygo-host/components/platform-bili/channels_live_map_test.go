// 直播类渠道映射测试（离线）：房间详情聚合/热门/分区目录/分区列房登录门禁。

package bili

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestLiveRoomMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "getInfoByRoom") && strings.Contains(req.URL.String(), "room_id=6") {
			return jsonResponse(http.StatusOK, `{"code":0,"data":{
				"room_info":{"room_id":6,"title":"房间","cover":"https://i0.hdslb.com/r.jpg",
					"live_status":1,"online":123,"description":"介绍","live_start_time":1728800000},
				"anchor_info":{"base_info":{"uname":"主播"}}}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":-1,"message":"fail"}`), nil
	})
	ch := channelByKey(t, list, "bili:live")
	result := fetchSource(t, ch, platform.SourceInfo{"roomIds": "6, 7"})
	if len(result.Items) != 1 {
		t.Fatalf("单房间失败应被跳过: %d", len(result.Items))
	}
	item := result.Items[0]
	if item.ID != "bilibili:6" || item.RoomID == nil || *item.RoomID != "6" || item.Platform != "bilibili" {
		t.Errorf("id/roomId/platform = %q/%v/%q", item.ID, item.RoomID, item.Platform)
	}
	if item.Thumbnail == nil || *item.Thumbnail != "https://i0.hdslb.com/r.jpg" || item.Author == nil || item.Author.Name != "主播" {
		t.Errorf("thumbnail/author = %v/%+v", item.Thumbnail, item.Author)
	}
	if item.Introduction == nil || *item.Introduction != "介绍" {
		t.Errorf("introduction = %v", item.Introduction)
	}
	if item.ShowTime == nil || *item.ShowTime != "2024-10-13T06:13:20.000Z" {
		t.Errorf("showTime = %v", item.ShowTime)
	}
	if item.LiveStatus == nil || *item.LiveStatus != "live" || item.Online == nil || *item.Online != 123 {
		t.Errorf("liveStatus/online = %v/%v", item.LiveStatus, item.Online)
	}
	if item.FetchedAt != testNow {
		t.Errorf("fetchedAt = %d", item.FetchedAt)
	}
	mustContain(t, fs.find("getInfoByRoom").URL, "w_rid=", "直播签名")

	// 全部失败必须抛错（不能静默空列表）。
	_, _, failList := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":-1,"message":"fail"}`), nil
	})
	_, err := channelByKey(t, failList, "bili:live").GetSource(platform.SourceInfo{"roomIds": "6,7"}).Fetch(context.Background())
	if err == nil || err.Error() != "bili:live 全部房间拉取失败(2 个)" {
		t.Fatalf("err = %v", err)
	}
}

func TestLiveHotMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
			{"roomid":24022841,"title":"热门","cover":"https://i0.hdslb.com/h.jpg","uname":"主播","online":999}
		],"has_more":1}}`), nil
	})
	ch := channelByKey(t, list, "bili:live:hot")
	page := fetchMorePage(t, ch, platform.SourceInfo{}, "2")
	item := page.Items[0]
	if item.ID != "bilibili:24022841" || item.URL == nil || *item.URL != "https://live.bilibili.com/24022841" {
		t.Errorf("id/url = %q/%v", item.ID, item.URL)
	}
	if item.LiveStatus == nil || *item.LiveStatus != "live" || item.Online == nil || *item.Online != 999 {
		t.Errorf("liveStatus/online = %v/%v", item.LiveStatus, item.Online)
	}
	req := fs.find("second/getListByArea")
	mustContain(t, req.URL, "page=2", "页码")
	mustContain(t, req.URL, "w_rid=", "wbi 签名")
	mustContain(t, req.Header.Get("cookie"), "buvid3=ANON_B3", "buvid 必带")
}

func TestLiveAreasMapping(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":[
			{"id":2,"name":"网游","list":[{"id":86,"name":"英雄联盟"},{"id":"","name":"空"}]},
			{"id":"","name":"空分区"}
		]}`), nil
	})
	ch := channelByKey(t, list, "bili:live:areas")
	result := fetchSource(t, ch, platform.SourceInfo{})
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2（空 id 顶层分区被跳过）", len(result.Items))
	}
	top := result.Items[0]
	if top.ID != "bili-area:2" || top.Title != "网游" || top.Summary == nil || *top.Summary != "2 个子分区" {
		t.Errorf("顶层 = %+v", top)
	}
	if top.CategoryChannel == nil || *top.CategoryChannel != "bili:live:area" ||
		top.CategoryInfo["parentAreaId"] != "2" || top.CategoryInfo["areaId"] != "0" {
		t.Errorf("category 元数据 = %v/%v", top.CategoryChannel, top.CategoryInfo)
	}
	if top.URL == nil || *top.URL != "https://live.bilibili.com/p/eden/area-tags?parentAreaId=2&areaId=0" {
		t.Errorf("url = %v", top.URL)
	}
	child := result.Items[1]
	if child.ID != "bili-area:2:86" || child.Summary == nil || *child.Summary != "网游 · 子分区" {
		t.Errorf("子分区 = %+v", child)
	}
	mustContain(t, fs.find("Area/getList").URL, "need_entrance=1", "目录请求")
}

func TestLiveAreaLoginGate(t *testing.T) {
	_, fs, list := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
	})
	ch := channelByKey(t, list, "bili:live:area")
	_, err := ch.GetSource(platform.SourceInfo{"parentAreaId": "2"}).Fetch(context.Background())
	if err == nil || err.Error() != "B站直播分区列房需要登录：请到「账号与凭证」扫码登录" {
		t.Fatalf("未登录 err = %v", err)
	}
	if fs.find("second/getList") != nil {
		t.Error("未登录不应发列房请求（该端点未登录必 -352）")
	}
	if _, err := ch.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil ||
		err.Error() != "bili:live:area 需要 parentAreaId 参数" {
		t.Errorf("缺参数 err = %v", err)
	}
}

func TestLiveAreaTranslationAndItems(t *testing.T) {
	respond := func(req *http.Request) (*http.Response, error) {
		url := req.URL.String()
		switch {
		case strings.Contains(url, "/lol"):
			return jsonResponse(http.StatusOK, `<html>"access_id":"WEB\\ID"</html>`), nil
		case strings.Contains(url, "second/getList") && strings.Contains(url, "page=1"):
			return jsonResponse(http.StatusOK, `{"code":0,"data":{"list":[
				{"roomid":5,"title":"分区房","cover":"https://i0.hdslb.com/a.jpg","uname":"主播","online":7}
			],"has_more":1}}`), nil
		default:
			return jsonResponse(http.StatusOK, `{"code":0,"data":{}}`), nil
		}
	}
	_, fs, list := newServiceChannel(t, respond)
	ch := channelByKey(t, list, "bili:live:area")
	page := fetchMorePage(t, ch, platform.SourceInfo{"parentAreaId": "2", "areaId": "86", "cookie": "SESSDATA=abc"}, "1")
	if len(page.Items) != 1 || page.Items[0].ID != "bilibili:5" {
		t.Fatalf("items = %+v", page.Items)
	}
	if page.HasMore == nil || !*page.HasMore {
		t.Errorf("hasMore = %v", page.HasMore)
	}
	req := fs.find("second/getList")
	mustContain(t, req.URL, "parent_area_id=2", "父分区")
	mustContain(t, req.URL, "area_id=86", "子分区")
	mustContain(t, req.URL, "w_webid=WEBID", "w_webid 去反斜杠")

	// 两次 -352 → 翻译成登录引导。
	_, _, list352 := newServiceChannel(t, func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "/lol") {
			return jsonResponse(http.StatusOK, `<html>"access_id":"WEBID"</html>`), nil
		}
		return jsonResponse(http.StatusOK, `{"code":-352,"message":"风控"}`), nil
	})
	_, err := channelByKey(t, list352, "bili:live:area").
		GetSource(platform.SourceInfo{"parentAreaId": "2", "cookie": "SESSDATA=abc"}).Fetch(context.Background())
	if err == nil || err.Error() != "B站直播分区列房需要登录：请到「账号与凭证」扫码登录" {
		t.Fatalf("-352 翻译 err = %v", err)
	}
}
