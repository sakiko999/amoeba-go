// 回放链路测试:时长/point_id 解析、渠道分P展开与分页、VOD 取流签名链
// (JS 第一参数 = point_id)、VOD 弹幕分页与颜色映射、show 路由。
// fixture 依据 docs/douyu-replay-research.md 的实测字段形态手工构造。

package douyu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mygo-host/platform"
)

func TestParseVideoDuration(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"113:21", 6801, true},
		{"02:00:02", 7202, true},
		{"16:47", 1007, true},
		{"0", 0, false}, // 0 = 未知时长,省略
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got := parseVideoDuration(c.in)
		if !c.ok {
			if got != nil {
				t.Errorf("parseVideoDuration(%q) = %v, want nil", c.in, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("parseVideoDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestShowPointIDExtract(t *testing.T) {
	page := `<html><script>window.$DATA = {"room":{"point_id":8512345678,"other":1}}</script></html>`
	match := pointIDRe.FindStringSubmatch(page)
	if match == nil || match[1] != "8512345678" {
		t.Errorf("point_id 提取失败: %v", match)
	}
}

func TestVodColorOf(t *testing.T) {
	if got := vodColorOf(0); got != nil {
		t.Errorf("col=0 应省略(默认白),got %v", *got)
	}
	if got := vodColorOf(7); got == nil || *got != "#FF5654" {
		t.Errorf("col=7 = %v, want #FF5654", got)
	}
	if got := vodColorOf(1); got == nil || *got != "#ff0000" {
		t.Errorf("col=1 复用直播色,got %v", got)
	}
	if got := vodColorOf(99); got != nil {
		t.Errorf("col=99 应省略,got %v", *got)
	}
}

// replayServer 覆写 baseURL/vodBaseURL 的 fake 服务(betard/列表/分P/详情/
// homeH5Enc/取流/弹幕 按路径分发)。
func replayServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldBase, oldVod := baseURL, vodBaseURL
	baseURL, vodBaseURL = srv.URL, srv.URL
	t.Cleanup(func() {
		baseURL, vodBaseURL = oldBase, oldVod
		srv.Close()
	})
}

func withReplayFixtures(t *testing.T, barragePages func(hit int) (string, int)) *[]string {
	t.Helper()
	posts := &[]string{}
	replayServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/betard/71415"):
			fmt.Fprintf(w, `{"error":0,"room":{"room_id":71415,"up_id":"5JjALNBWgwXr","room_name":"寅子"}}`)
		case strings.Contains(r.URL.Path, "authorShowVideoList"):
			fmt.Fprint(w, `{"error":0,"data":{"count":45,"list":[
				{"title":"上午场&amp;下棋","start_time":1760000000,"is_replay":1,"re_num":2,
				 "video_list":[{"hash_id":"JmbBMkRlNzN740XA","vid":851111,"video_duration":0,"video_str_duration":"02:00:02"}]},
				{"title":"下午场","start_time":0,"is_replay":1,"re_num":1,
				 "video_list":[{"hash_id":"85BAvqGOnLqMG4Lm","vid":852222,"video_duration":0,"video_str_duration":"16:47"}]}
			]}}`)
		case strings.Contains(r.URL.Path, "getShowReplayList"):
			fmt.Fprint(w, `{"error":0,"data":{"list":[
				{"hash_id":"JmbBMkRlNzN740XA","vid":851111,"video_duration":"02:00:02","rank":1},
				{"hash_id":"ZnXCp2PartTwo00","vid":851112,"video_duration":"01:13:21","rank":2}
			]}}`)
		case strings.Contains(r.URL.Path, "/show/"):
			fmt.Fprint(w, `<html><script>window.$DATA={"room":{"point_id":8512345678}}</script></html>`)
		case strings.Contains(r.URL.Path, "homeH5Enc"):
			fmt.Fprint(w, `{"error":0,"data":{"room8512345678":"var v2201 = function(...){}"}}`)
		case strings.Contains(r.URL.Path, "getStreamUrlWeb"):
			body := readBody(t, r)
			*posts = append(*posts, body)
			fmt.Fprint(w, `{"error":0,"data":{"thumb_video":{
				"normal":{"url":"http://vod-tf.douyucdn.cn/v/normal.m3u8?tlink=x","name":"480P","bit_rate":0},
				"super":{"url":"https://vod-tf.douyucdn.cn/v/super.m3u8?tlink=y","name":"蓝光10M","bit_rate":10000},
				"high":{"url":"http://vod-tf.douyucdn.cn/v/high.m3u8?tlink=z","name":"高清","bit_rate":4000}
			}}}`)
		case strings.Contains(r.URL.Path, "getBarrageList"):
			body, end := barragePages(len(*posts))
			fmt.Fprint(w, body)
			_ = end
		default:
			http.NotFound(w, r)
		}
	})
	return posts
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	buf := make([]byte, 4096)
	n, _ := r.Body.Read(buf)
	return string(buf[:n])
}

func TestReplayChannelFetch(t *testing.T) {
	withReplayFixtures(t, nil)
	sc := newTestContext()
	ch := newReplayChannel(sc)
	result, err := ch.GetSource(platform.SourceInfo{"roomId": "71415"}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 场次 1(re_num=2 → 分P展开 2 条)+ 场次 2(单分P 1 条)。
	if len(result.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(result.Items))
	}
	first, second, third := result.Items[0], result.Items[1], result.Items[2]
	if first.ID != "douyu:vod:JmbBMkRlNzN740XA" || first.Title != "上午场&下棋(P1/2)" {
		t.Errorf("first = %s / %q", first.ID, first.Title)
	}
	if first.Duration == nil || *first.Duration != 7202 {
		t.Errorf("first.duration = %v, want 7202", first.Duration)
	}
	if first.PublishedAt == nil || *first.PublishedAt != 1760000000000 {
		t.Errorf("first.publishedAt = %v", first.PublishedAt)
	}
	if second.ID != "douyu:vod:ZnXCp2PartTwo00" || second.Title != "上午场&下棋(P2/2)" {
		t.Errorf("second = %s / %q", second.ID, second.Title)
	}
	if second.Duration == nil || *second.Duration != 4401 {
		t.Errorf("second.duration = %v, want 4401(01:13:21)", second.Duration)
	}
	if third.PublishedAt != nil {
		t.Errorf("start_time=0 应省略 publishedAt")
	}
	if third.URL == nil || !strings.Contains(*third.URL, "/show/85BAvqGOnLqMG4Lm") {
		t.Errorf("third.url = %v", third.URL)
	}
}

func TestReplayChannelPaging(t *testing.T) {
	withReplayFixtures(t, nil)
	source := newReplayChannel(newTestContext()).GetSource(platform.SourceInfo{"roomId": "71415"}).(platform.PageableSource)
	// Fetch 已拉 page 1;首次 FetchMore 从 First=2 起(不重复第 1 页)。
	// count=45:page=2 → hasMore 且游标 3;page=3 → 60 ≥ 45 无更多。
	page1, err := source.FetchMore(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if page1.HasMore == nil || !*page1.HasMore || page1.Cursor == nil || *page1.Cursor != "3" {
		t.Errorf("page1 = %+v, want hasMore+cursor 3", page1)
	}
	page3, err := source.FetchMore(context.Background(), "3", 40)
	if err != nil {
		t.Fatal(err)
	}
	if page3.HasMore == nil || *page3.HasMore {
		t.Errorf("page3.hasMore 应为 false(3*20 ≥ 45)")
	}
}

func TestResolveVideoStreamsFlow(t *testing.T) {
	capture := &jsCapture{}
	withReplayFixtures(t, nil)
	sc := testContextWithJS(platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) {
		capture.fn, capture.args = fn, args
		return "signed-body", nil
	}))
	streams, err := resolveVideoStreams(context.Background(), sc, "85BAvqGOnLqMG4Lm")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 3 {
		t.Fatalf("streams = %d, want 3", len(streams))
	}
	// bit_rate 降序:蓝光10M > 高清 > 480P;http 统一升 https。
	if *streams[0].Quality != "蓝光10M" || !strings.HasPrefix(streams[0].URL, "https://") {
		t.Errorf("top stream = %v %v", *streams[0].Quality, streams[0].URL)
	}
	if !strings.HasPrefix(streams[2].URL, "https://") {
		t.Errorf("480P URL 应升 https: %s", streams[2].URL)
	}
	if *streams[0].Format != "hls" {
		t.Errorf("format = %s", *streams[0].Format)
	}
	// 签名第一参数必须是详情页 point_id,而非 hash。
	if len(capture.args) < 1 || capture.args[0] != "8512345678" {
		t.Errorf("sign args[0] = %v, want point_id 8512345678", capture.args)
	}
}

func TestVodDanmakuPaging(t *testing.T) {
	page := 0
	withReplayFixtures(t, func(hit int) (string, int) {
		page++
		if page == 1 {
			return `{"error":0,"data":{"list":[
				{"ctt":"第一页","nn":"a","tl":1000,"col":7,"sts":1760000000},
				{"ctt":"白字","nn":"b","tl":2000,"col":0,"sts":1760000001}
			],"end_time":500}}`, 500
		}
		return `{"error":0,"data":{"list":[
			{"ctt":"第二页","nn":"c","tl":60000,"col":13,"sts":1760000100}
		],"end_time":-1}}`, -1
	})
	sc := newTestContext()
	var mu sync.Mutex
	all := []platform.DanmakuItem{}
	stop := vodDanmakuStream(sc, "85BAvqGOnLqMG4Lm")(func(items []platform.DanmakuItem) {
		mu.Lock()
		all = append(all, items...)
		mu.Unlock()
	})
	defer stop()
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		n := len(all)
		mu.Unlock()
		if n == 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("弹幕拉取超时,got %d 条", n)
		case <-time.After(20 * time.Millisecond):
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if all[0].Text != "第一页" || all[0].TimeMs == nil || *all[0].TimeMs != 1000 {
		t.Errorf("item0 = %+v", all[0])
	}
	if all[1].Color != nil {
		t.Errorf("col=0 应省略 color")
	}
	if all[2].Color == nil || *all[2].Color != "#3D9BFF" {
		t.Errorf("col=13 = %v", all[2].Color)
	}
}

func TestShowRoutes(t *testing.T) {
	pb := newPlayback(newTestContext())
	if id, kind, ok := platform.MatchRoute(pb.Routes, "https://v.douyu.com/show/85BAvqGOnLqMG4Lm"); !ok || id != "85BAvqGOnLqMG4Lm" || kind != "video" {
		t.Errorf("show 路由 = %q/%q/%v", id, kind, ok)
	}
	if _, kind, ok := platform.MatchRoute(pb.Routes, "https://v.douyu.com/vmobile/abc123XYZ"); !ok || kind != "video" {
		t.Errorf("vmobile 路由 kind = %q ok=%v", kind, ok)
	}
	// 直播 URL 不被回放路由误命中(反向也成立)。
	if id, kind, ok := platform.MatchRoute(pb.Routes, "https://www.douyu.com/71415"); !ok || id != "71415" || kind != "live" {
		t.Errorf("直播路由 = %q/%q/%v", id, kind, ok)
	}
}
