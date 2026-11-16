// 渠道抓取测试:httptest 模拟热门/分区/主播/目录端点,覆盖过滤、翻页与错误隔离。

package douyu

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"mygo-host/platform"
)

func TestLiveChannelMultiRoom(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/betard/1":
			io.WriteString(w, `{"room":{"room_name":"房间A","room_id":1,"show_status":1,"videoLoop":0,"room_pic":"pic","owner_name":"主播","room_biz_all":{"hot":50}}}`)
		case "/betard/2":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})
	sc := testContextWithJS(nil)
	source := newLiveChannel(sc).GetSource(platform.SourceInfo{"roomIds": "1, 2"})
	result, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "douyu:1" {
		t.Fatalf("items = %+v", result.Items)
	}
	if result.Items[0].FetchedAt != testNow || *result.Items[0].Online != 50 {
		t.Errorf("item = %+v", result.Items[0])
	}
	if _, ok := source.(platform.PageableSource); ok {
		t.Error("live:douyu 不应暴露 fetchMore")
	}
}

func TestLiveChannelAllRoomsFail(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	sc := testContextWithJS(nil)
	source := newLiveChannel(sc).GetSource(platform.SourceInfo{"roomIds": "3,4"})
	_, err := source.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "全部房间拉取失败(2 个)") {
		t.Fatalf("err = %v", err)
	}
	if _, err := newLiveChannel(sc).GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil {
		t.Fatal("缺少 roomIds 应报错")
	}
}

func TestHotChannelPagination(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/japi/weblist/apinc/allpage/6/1":
			io.WriteString(w, `{"data":{"rl":[{"rid":1,"rn":"A","type":1,"ol":"133.1万"},{"rid":2,"rn":"广告","type":2}]}}`)
		case "/japi/weblist/apinc/allpage/6/2":
			io.WriteString(w, `{"data":{"rl":[]}}`)
		default:
			http.NotFound(w, r)
		}
	})
	sc := testContextWithJS(nil)
	source := newHotChannel(sc).GetSource(platform.SourceInfo{})
	first, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || *first.Items[0].Online != 1331000 {
		t.Fatalf("first = %+v", first.Items)
	}
	pageable, ok := source.(platform.PageableSource)
	if !ok {
		t.Fatal("hot 应支持翻页")
	}
	page, err := pageable.FetchMore(context.Background(), "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.Cursor != nil || page.HasMore == nil || *page.HasMore {
		t.Errorf("page = %+v", page)
	}
}

func TestCategoryChannel(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gapi/rkc/directory/mixList/2_1/1":
			io.WriteString(w, `{"data":{"rl":[{"rid":11,"rn":"A","type":1,"ol":"1234"},{"rid":12,"rn":"广告","type":2}],"pgcnt":2}}`)
		case "/gapi/rkc/directory/mixList/2_1/2":
			io.WriteString(w, `{"data":{"rl":[{"rid":13,"rn":"B","type":1}],"pgcnt":2}}`)
		default:
			http.NotFound(w, r)
		}
	})
	sc := testContextWithJS(nil)
	channel := newCategoryChannel(sc)
	if _, err := channel.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil {
		t.Fatal("缺少 areaId 应报错")
	}
	source := channel.GetSource(platform.SourceInfo{"areaId": " 1 "})
	first, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "douyu:11" {
		t.Fatalf("first = %+v", first.Items)
	}
	page, err := source.(platform.PageableSource).FetchMore(context.Background(), "", -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Cursor != nil {
		t.Errorf("page = %+v", page)
	}
}

func TestAnchorChannel(t *testing.T) {
	var gotQuery string
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/japi/search/api/searchUser" {
			http.NotFound(w, r)
			return
		}
		gotQuery = r.URL.RawQuery
		io.WriteString(w, `{"data":{"relateUser":[{"anchorInfo":{"rid":10,"nickName":"主播","isLive":1,"roomType":0}}]}}`)
	})
	sc := testContextWithJS(nil)
	channel := newAnchorChannel(sc)
	if _, err := channel.GetSource(platform.SourceInfo{}).Fetch(context.Background()); err == nil {
		t.Fatal("缺少 keyword 应报错")
	}
	items, err := channel.GetSource(platform.SourceInfo{"keyword": " 星 海 "}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 1 || items.Items[0].ID != "douyu:10" || items.Items[0].Online != nil {
		t.Fatalf("items = %+v", items.Items)
	}
	if !strings.Contains(gotQuery, "kw=%E6%98%9F%20%E6%B5%B7") ||
		!strings.Contains(gotQuery, "page=1&pageSize=20&filterType=1") {
		t.Errorf("query = %s", gotQuery)
	}
}

func TestSearchRooms(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/japi/search/api/searchShow" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"data":{"relateShow":[{"rid":31,"roomName":"房","isLive":1,"roomType":0,"roomSrc":"src","nickName":"主播","hot":"10万+"},{"rid":32,"roomName":"录播","isLive":0,"roomType":0}]}}`)
	})
	items, err := searchRooms(context.Background(), testContextWithJS(nil), "房", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || *items[0].Online != 100000 || *items[1].LiveStatus != platform.LiveStatusOffline {
		t.Fatalf("items = %+v", items)
	}
}

func TestCategoriesChannel(t *testing.T) {
	withDouyuServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cate/list" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `{"data":{"cate1Info":[{"cate1Id":1,"cate1Name":"网游"},{"cate1Id":2,"cate1Name":"手游"},{"cate1Id":"","cate1Name":"空"}],"cate2Info":[{"cate1Id":1,"cate2Id":10,"cate2Name":"英雄联盟"},{"cate1Id":2,"cate2Id":20,"cate2Name":"王者荣耀"}]}}`)
	})
	sc := testContextWithJS(nil)
	source := newCategoriesChannel(sc).GetSource(platform.SourceInfo{})
	if _, ok := source.(platform.PageableSource); ok {
		t.Error("categories 不应暴露 fetchMore")
	}
	result, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %+v", result.Items)
	}
	if result.Items[0].ID != "douyu-area:10" || *result.Items[0].Summary != "网游" ||
		result.Items[0].FetchedAt != testNow {
		t.Errorf("items[0] = %+v", result.Items[0])
	}
}

func TestEncodeURIComponent(t *testing.T) {
	if got := encodeURIComponent("a b/中"); got != "a%20b%2F%E4%B8%AD" {
		t.Errorf("encode = %s", got)
	}
}
