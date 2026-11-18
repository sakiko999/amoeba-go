// ChannelMeta JSON 形状回归:空 sourceInfoTpl([])与空 defaultInfo({})是
// TS channelMeta 会输出的占位(多个平台的 hot/categories 渠道依赖),
// nil 才表示「未声明」。此形状曾被 omitempty 吞掉,回归锁定。

package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type shapeChannel struct{}

func (shapeChannel) Key() string                      { return "ch" }
func (shapeChannel) Name() string                     { return "渠道" }
func (shapeChannel) Kind() Kind                       { return KindLive }
func (shapeChannel) SourceInfoTpl() []SourceInfoField { return []SourceInfoField{} }
func (shapeChannel) DefaultInfo() SourceInfo          { return SourceInfo{} }
func (shapeChannel) GetSource(SourceInfo) DataSource  { return nil }

type absentChannel struct{}

func (absentChannel) Key() string                      { return "absent" }
func (absentChannel) Name() string                     { return "无参渠道" }
func (absentChannel) Kind() Kind                       { return KindVideo }
func (absentChannel) SourceInfoTpl() []SourceInfoField { return nil }
func (absentChannel) DefaultInfo() SourceInfo          { return nil }
func (absentChannel) GetSource(SourceInfo) DataSource  { return nil }

func TestChannelMetaJSONShape(t *testing.T) {
	ops := FeedOps(FeedSpec{Channels: []DataChannel{shapeChannel{}, absentChannel{}}})
	result, err := ops["channels"].Invoke(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"sourceInfoTpl":[]`) {
		t.Errorf("空 sourceInfoTpl 应输出 []: %s", text)
	}
	if !strings.Contains(text, `"defaultInfo":{}`) {
		t.Errorf("空 defaultInfo 应输出 {}: %s", text)
	}
	if strings.Contains(text, `"absent","name":"无参渠道","kind":"video","sourceInfoTpl"`) {
		t.Errorf("nil 占位应省略: %s", text)
	}
}

// 无 Pageable 的 MakeChannel 渠道,带 cursor 请求应报「不支持翻页」而非 panic。
func TestMakeChannelWithoutPageable(t *testing.T) {
	ch := MakeChannel(ChannelSpec{
		Key:  "single",
		Name: "单页",
		Kind: KindVideo,
		FetchPage: func(ctx context.Context, _ SourceInfo, _ int) (PageItems, error) {
			return PageItems{Items: []Item{{ID: "x", SourceID: "single", Kind: KindVideo, Title: "X", FetchedAt: 1}}}, nil
		},
	})
	ops := FeedOps(FeedSpec{Channels: []DataChannel{ch}})
	if _, err := ops["fetch"].Invoke(t.Context(), []byte(`{"channel":"single","cursor":"2"}`)); err == nil {
		t.Fatal("单页渠道带 cursor 应报错")
	} else if !strings.Contains(err.Error(), "不支持翻页") {
		t.Fatalf("错误应为不支持翻页: %v", err)
	}
	// 无 cursor 的正常路径不受影响
	if _, err := ops["fetch"].Invoke(t.Context(), []byte(`{"channel":"single"}`)); err != nil {
		t.Fatal(err)
	}
}
