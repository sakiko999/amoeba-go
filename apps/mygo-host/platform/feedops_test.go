// feedops 协议测试:fetch/cursor 翻页/passive 与 play 路由语义(对齐 TS tests/feed 语义)。

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
)

type fakeChannel struct{ key string }

func (c *fakeChannel) Key() string                      { return c.key }
func (c *fakeChannel) Name() string                     { return "fake" }
func (c *fakeChannel) Kind() Kind                       { return KindVideo }
func (c *fakeChannel) SourceInfoTpl() []SourceInfoField { return nil }
func (c *fakeChannel) DefaultInfo() SourceInfo          { return nil }
func (c *fakeChannel) GetSource(info SourceInfo) DataSource {
	return &fakeSource{}
}

type fakeSource struct{}

func (s *fakeSource) Fetch(ctx context.Context) (FetchResult, error) {
	return FetchResult{Items: []Item{{ID: "a", SourceID: "fake", Kind: KindVideo, Title: "A", FetchedAt: 1}}}, nil
}

func (s *fakeSource) FetchMore(ctx context.Context, cursor string, existing int) (FetchPage, error) {
	if cursor == "2" {
		return FetchPage{Items: nil}, nil // 第二页空 = 没有更多
	}
	if cursor == "err" {
		return FetchPage{}, errors.New("boom")
	}
	next := "2"
	return FetchPage{Items: []Item{{ID: "b", SourceID: "fake", Kind: KindVideo, Title: "B", FetchedAt: 2}}, Cursor: &next}, nil
}

func callOps(t *testing.T, ops Ops, name string, params string) (any, error) {
	t.Helper()
	return ops[name].Invoke(context.Background(), json.RawMessage(params))
}

func TestFeedOpsChannels(t *testing.T) {
	ops := FeedOps(FeedSpec{Channels: []DataChannel{&fakeChannel{key: "fake"}}})
	result, err := callOps(t, ops, "channels", "{}")
	if err != nil {
		t.Fatal(err)
	}
	metas := result.([]ChannelMeta)
	if len(metas) != 1 || metas[0].Key != "fake" || metas[0].Kind != "video" {
		t.Errorf("channels = %+v", metas)
	}
}

func TestFeedOpsFetchFlow(t *testing.T) {
	ops := GuardOps("test", FeedOps(FeedSpec{Channels: []DataChannel{&fakeChannel{key: "fake"}}}))

	// 未知渠道
	if _, err := callOps(t, ops, "fetch", `{"channel":"nope"}`); err == nil {
		t.Error("expected unknown channel error")
	}
	// 首页
	first, err := callOps(t, ops, "fetch", `{"channel":"fake"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.(FetchResult).Items) != 1 {
		t.Errorf("fetch = %+v", first)
	}
	// 翻页带 cursor 与 existing
	page, err := callOps(t, ops, "fetch", `{"channel":"fake","cursor":"1","existing":5}`)
	if err != nil {
		t.Fatal(err)
	}
	fp := page.(FetchPage)
	if len(fp.Items) != 1 || fp.Cursor == nil || *fp.Cursor != "2" {
		t.Errorf("fetchMore = %+v", fp)
	}
	// 空页 = 无 cursor
	empty, err := callOps(t, ops, "fetch", `{"channel":"fake","cursor":"2"}`)
	if err != nil {
		t.Fatal(err)
	}
	if empty.(FetchPage).Cursor != nil {
		t.Errorf("empty page cursor = %v", empty.(FetchPage).Cursor)
	}
	// 渠道错误被包装成 PlatformOpError
	_, err = callOps(t, ops, "fetch", `{"channel":"fake","cursor":"err"}`)
	var opErr *PlatformOpError
	if !errors.As(err, &opErr) || opErr.Platform != "test" {
		t.Errorf("err = %v", err)
	}
	// 无 playback → play 不注册
	if _, ok := ops["play"]; ok {
		t.Error("play should not exist without playback")
	}
}

func TestFeedOpsPlayRoute(t *testing.T) {
	pb := &Playback{
		Platform: "fake",
		Routes: []Route{{
			Re:        compileRoute(t, `^https://fake/v/(\w+)$`),
			Kind:      "video",
			ExtractID: func(url string) (string, bool) { return subMatch(url), true },
		}},
		ResolveVideoPlay: func(ctx context.Context, id string, info ResolveInfo) (*VideoPlayResult, error) {
			return &VideoPlayResult{Streams: []Stream{{URL: "https://cdn/" + id}}}, nil
		},
		ResolveLivePlay: func(ctx context.Context, id string, info ResolveInfo) ([]Stream, error) {
			return []Stream{{URL: "https://live/" + id}}, nil
		},
	}
	ops := FeedOps(FeedSpec{Channels: []DataChannel{&fakeChannel{key: "fake"}}, Playback: pb})

	// url 路由 → video 解析
	result, err := callOps(t, ops, "play", `{"url":"https://fake/v/abc"}`)
	if err != nil {
		t.Fatal(err)
	}
	play := result.(PlayOpResult)
	if play.ID != "abc" || len(play.Streams) != 1 || play.Streams[0].URL != "https://cdn/abc" {
		t.Errorf("play = %+v", play)
	}
	// id + kind=live → 直播解析
	live, err := callOps(t, ops, "play", `{"id":"42","kind":"live"}`)
	if err != nil {
		t.Fatal(err)
	}
	if live.(PlayOpResult).Streams[0].URL != "https://live/42" {
		t.Errorf("live = %+v", live)
	}
	// 无 danmaku 能力 → 不注册
	if _, ok := ops["danmaku"]; ok {
		t.Error("danmaku should not exist without GetDanmaku")
	}
	// 无法路由的 url 报错
	if _, err := callOps(t, ops, "play", `{"url":"https://other/x"}`); err == nil {
		t.Error("expected route error")
	}
}

func compileRoute(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return re
}

// subMatch 取第一个捕获组(测试路由 ExtractID 用)。
func subMatch(url string) string {
	re := regexp.MustCompile(`^https://fake/v/(\w+)$`)
	m := re.FindStringSubmatch(url)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
