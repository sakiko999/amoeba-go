// 身份解析(搜索条目 → 主页目标)与视频列表的渠道路由(网关假实现捕获 op/params)。

package profile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mygo-host/platform"
)

type stubOpener struct {
	opened []ProfileTarget
	target map[string]ProfileTarget
}

func (o *stubOpener) OpenProfile(_ string, target ProfileTarget) error {
	o.opened = append(o.opened, target)
	return nil
}

func (o *stubOpener) ProfileTarget(instanceID string) (ProfileTarget, bool) {
	t, ok := o.target[instanceID]
	return t, ok
}

func newSvc(t *testing.T, opener Opener) (*Service, platform.Service) {
	t.Helper()
	svc, comp, err := NewService(opener)
	if err != nil {
		t.Fatal(err)
	}
	return svc, comp
}

// fakeGateway 同时实现 Registrar 与 OpCaller(与 *gateway.Gateway 同面)。
type fakeGateway struct {
	result any
	op     string
	params json.RawMessage
}

func (g *fakeGateway) Register(platform.Spec) error { return nil }

func (g *fakeGateway) Call(_ context.Context, op string, params json.RawMessage) (any, error) {
	g.op = op
	g.params = params
	return g.result, nil
}

func liveItem(ns string) platform.Item {
	return platform.Item{
		ID: "x", SourceID: "test", Kind: platform.KindLive, Title: "房间标题", FetchedAt: 1,
		URL:        platform.Str("https://example.com/room/1"),
		Thumbnail:  platform.Str("https://cdn/cover.jpg"),
		Author:     &platform.Author{Name: "主播甲", Avatar: platform.Str("https://cdn/face.jpg"), Handle: platform.Str("42")},
		Platform:   ns,
		RoomID:     platform.Str("1"),
		LiveStatus: statusPtr(platform.LiveStatusLive),
		Online:     platform.Int(123),
	}
}

func statusPtr(s platform.LiveStatus) *platform.LiveStatus { return &s }

func TestTargetOf(t *testing.T) {
	cases := []struct {
		name    string
		ns      string
		item    platform.Item
		wantErr string
		check   func(t *testing.T, got ProfileTarget)
	}{
		{
			name: "bili 直播条目", ns: "bili", item: liveItem("bilibili"),
			check: func(t *testing.T, got ProfileTarget) {
				if got.UID != "42" || got.RoomID != "1" || got.Name != "主播甲" {
					t.Errorf("target = %+v", got)
				}
				if got.LiveStatus != "live" || got.LiveOnline != 123 || got.LiveCover == "" {
					t.Errorf("live 快照 = %+v", got)
				}
			},
		},
		{
			name: "twitch 频道搜索命中(无 roomId,从 URL 取 login)", ns: "twitch",
			item: platform.Item{ID: "tw-a", Title: "SomeStreamer", FetchedAt: 1,
				URL: platform.Str("https://www.twitch.tv/some_streamer")},
			check: func(t *testing.T, got ProfileTarget) {
				if got.RoomID != "some_streamer" || got.Name != "SomeStreamer" {
					t.Errorf("target = %+v", got)
				}
			},
		},
		{
			name: "youtube 频道条目(channelId 从 URL 取)", ns: "youtube",
			item: platform.Item{ID: "ch-UC1", Title: "频道", FetchedAt: 1,
				URL: platform.Str("https://www.youtube.com/channel/UC123abc")},
			check: func(t *testing.T, got ProfileTarget) {
				if got.UID != "UC123abc" {
					t.Errorf("uid = %q, want UC123abc", got.UID)
				}
			},
		},
		{
			name: "douyu 直播条目", ns: "douyu", item: liveItem("douyu"),
			check: func(t *testing.T, got ProfileTarget) {
				if got.RoomID != "1" || got.Avatar == "" {
					t.Errorf("target = %+v", got)
				}
			},
		},
		{
			name: "weibo 用户条目(author.handle)", ns: "weibo",
			item: platform.Item{ID: "wbu-1", Title: "博主", FetchedAt: 1,
				URL:    platform.Str("https://weibo.com/u/123"),
				Author: &platform.Author{Name: "博主", Handle: platform.Str("123")}},
			check: func(t *testing.T, got ProfileTarget) {
				if got.UID != "123" || got.Name != "博主" {
					t.Errorf("target = %+v", got)
				}
			},
		},
		{
			name: "xhs 用户条目(URL 解析)", ns: "xhs",
			item: platform.Item{ID: "xu-1", Title: "小红书用户", FetchedAt: 1,
				URL: platform.Str("https://www.xiaohongshu.com/user/profile/0123456789abcdef01234567")},
			check: func(t *testing.T, got ProfileTarget) {
				if got.UID != "0123456789abcdef01234567" {
					t.Errorf("uid = %q", got.UID)
				}
			},
		},
		{
			name: "douyin 用户条目(URL 解析)", ns: "douyin",
			item: platform.Item{ID: "dyu-1", Title: "抖音用户", FetchedAt: 1,
				URL: platform.Str("https://www.douyin.com/user/MS4wLjABAAAAx")},
			check: func(t *testing.T, got ProfileTarget) {
				if got.UID != "MS4wLjABAAAAx" {
					t.Errorf("uid = %q", got.UID)
				}
			},
		},
		{
			name: "weibo 缺用户身份", ns: "weibo",
			item:    platform.Item{Title: "x", URL: platform.Str("https://weibo.com/")},
			wantErr: "缺少",
		},
		{name: "缺少标识", ns: "bili", item: platform.Item{Title: "x"}, wantErr: "缺少"},
		{
			name: "twitch 无 URL 身份", ns: "twitch",
			item:    platform.Item{Title: "x", URL: platform.Str("https://www.twitch.tv/directory/game/x")},
			wantErr: "频道名",
		},
		{name: "不支持的平台", ns: "tmdb", item: liveItem("x"), wantErr: "不支持"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := targetOf(tc.ns, tc.item)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want 含 %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Platform != tc.ns {
				t.Errorf("platform = %q", got.Platform)
			}
			tc.check(t, got)
		})
	}
}

func TestOpenRecordsTarget(t *testing.T) {
	opener := &stubOpener{}
	svc, _ := newSvc(t, opener)
	if err := svc.Open(context.Background(), OpenParams{Namespace: "douyu", Item: liveItem("douyu")}); err != nil {
		t.Fatal(err)
	}
	if len(opener.opened) != 1 || opener.opened[0].Platform != "douyu" || opener.opened[0].RoomID != "1" {
		t.Errorf("opened = %+v", opener.opened)
	}
	if err := svc.Open(context.Background(), OpenParams{}); err == nil {
		t.Error("空 namespace 应报错")
	}
}

func TestVideosRoutesPerPlatform(t *testing.T) {
	cases := []struct {
		ns        string
		target    ProfileTarget
		wantOp    string
		wantKey   string
		wantValue string
	}{
		{"bili", ProfileTarget{Platform: "bili", UID: "42"}, "bili.fetch", "uid", "42"},
		{"douyu", ProfileTarget{Platform: "douyu", RoomID: "1"}, "douyu.fetch", "roomId", "1"},
		{"twitch", ProfileTarget{Platform: "twitch", RoomID: "streamer"}, "twitch.fetch", "login", "streamer"},
		{"youtube", ProfileTarget{Platform: "youtube", UID: "UC1"}, "youtube.fetch", "channelId", "UC1"},
	}
	for _, tc := range cases {
		t.Run(tc.ns, func(t *testing.T) {
			opener := &stubOpener{target: map[string]ProfileTarget{"profile-1": tc.target}}
			svc, comp := newSvc(t, opener)
			gw := &fakeGateway{result: platform.FetchPage{Items: []platform.Item{liveItem(tc.ns)}}}
			if err := comp.Activate(gw); err != nil {
				t.Fatal(err)
			}
			res, err := svc.Videos(context.Background(), VideosParams{InstanceID: "profile-1"})
			if err != nil {
				t.Fatal(err)
			}
			if gw.op != tc.wantOp {
				t.Errorf("op = %q, want %q", gw.op, tc.wantOp)
			}
			var payload struct {
				Channel string            `json:"channel"`
				Info    map[string]string `json:"info"`
			}
			if err := json.Unmarshal(gw.params, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Channel == "" || payload.Info[tc.wantKey] != tc.wantValue {
				t.Errorf("payload = %+v, want info[%s]=%s", payload, tc.wantKey, tc.wantValue)
			}
			if len(res.Items) != 1 {
				t.Errorf("items = %d, want 1", len(res.Items))
			}
		})
	}
}

func TestVideosErrors(t *testing.T) {
	opener := &stubOpener{target: map[string]ProfileTarget{
		"huya-1":  {Platform: "huya", RoomID: "1"},
		"weibo-1": {Platform: "weibo", UID: "1"},
	}}
	svc, comp := newSvc(t, opener)
	if err := comp.Activate(&fakeGateway{result: platform.FetchResult{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Videos(context.Background(), VideosParams{InstanceID: "huya-1"}); err == nil ||
		!strings.Contains(err.Error(), "虎牙") {
		t.Errorf("huya 应报不支持,得到 %v", err)
	}
	if _, err := svc.Videos(context.Background(), VideosParams{InstanceID: "weibo-1"}); err == nil ||
		!strings.Contains(err.Error(), "暂不支持") {
		t.Errorf("weibo 应报无视频源,得到 %v", err)
	}
	if _, err := svc.Videos(context.Background(), VideosParams{InstanceID: "missing"}); err == nil {
		t.Error("没有目标应报错")
	}
}

// FetchResult(非翻页渠道)也要归一成 VideosResult。
func TestVideosFetchResultNormalized(t *testing.T) {
	opener := &stubOpener{target: map[string]ProfileTarget{
		"p": {Platform: "douyu", RoomID: "1"},
	}}
	svc, comp := newSvc(t, opener)
	if err := comp.Activate(&fakeGateway{result: platform.FetchResult{
		Items: []platform.Item{liveItem("douyu")}, Total: platform.Int(7),
	}}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Videos(context.Background(), VideosParams{InstanceID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Total == nil || *res.Total != 7 {
		t.Errorf("result = %+v", res)
	}
}

// 未激活(网关调用面缺失)时 Videos 应报错而不是空指针。
func TestVideosRequiresActivation(t *testing.T) {
	opener := &stubOpener{target: map[string]ProfileTarget{"p": {Platform: "bili", UID: "42"}}}
	svc, _ := newSvc(t, opener)
	if _, err := svc.Videos(context.Background(), VideosParams{InstanceID: "p"}); err == nil {
		t.Error("未激活时 Videos 应报错")
	}
}
