// feed 协议(feedOps 的 Go 版):内容渠道 + 播放/弹幕,装配成
// channels / fetch / play / danmaku 四个 ops。协议是消费方约定:
// 平台可在协议结果上追加私有 op(services 层合并)。

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FeedSpec feed 协议输入。
type FeedSpec struct {
	Channels []DataChannel
	// Playback 播放/弹幕能力(无播放能力的平台省略)。
	Playback *Playback
}

// ChannelMeta channels op 的返回元素(静态渠道描述)。
type ChannelMeta struct {
	Key           string            `json:"key"`
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	Passive       bool              `json:"passive,omitempty"`
	SourceInfoTpl []SourceInfoField `json:"sourceInfoTpl,omitempty"`
	DefaultInfo   SourceInfo        `json:"defaultInfo,omitempty"`
}

type fetchParams struct {
	Channel  string     `json:"channel"`
	Info     SourceInfo `json:"info"`
	Cursor   *string    `json:"cursor"`
	Existing *int       `json:"existing"`
	Passive  bool       `json:"passive"`
}

type playParams struct {
	URL  string      `json:"url"`
	ID   string      `json:"id"`
	Kind string      `json:"kind"` // video | live
	Info ResolveInfo `json:"info"`
}

type danmakuParams struct {
	URL    string `json:"url"`
	ID     string `json:"id"`
	Kind   string `json:"kind"` // vod | live
	Cookie string `json:"cookie"`
}

// PlayOpResult play op 返回值。
type PlayOpResult struct {
	ID       string            `json:"id"`
	Streams  []Stream          `json:"streams"`
	Chapters []PlaybackChapter `json:"chapters,omitempty"`
}

// DanmakuOpResult danmaku op 返回值。Subscribe 只能进程内消费(网关 JSON
// 序列化时丢弃;消费方经类型断言取用),与 TS 返回函数对在进程内可用同语义。
type DanmakuOpResult struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind,omitempty"`
	Subscribe DanmakuStream `json:"-"`
}

// FeedOps 装配 feed 协议 ops。
func FeedOps(spec FeedSpec) Ops {
	byKey := map[string]DataChannel{}
	for _, c := range spec.Channels {
		byKey[c.Key()] = c
	}
	ops := Ops{
		"channels": {
			Description: "渠道清单(静态描述:key / 名称 / kind / 参数表)",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				metas := make([]ChannelMeta, 0, len(spec.Channels))
				for _, c := range spec.Channels {
					meta := ChannelMeta{Key: c.Key(), Name: c.Name(), Kind: string(c.Kind()), SourceInfoTpl: c.SourceInfoTpl(), DefaultInfo: c.DefaultInfo()}
					metas = append(metas, meta)
				}
				return metas, nil
			},
		},
		"fetch": {
			Description: "抓取渠道数据;cursor 翻页(existing=消费方已持有条数),passive=true 走页面被动采集",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				return fetchChannel(ctx, byKey, params)
			},
		},
	}
	if spec.Playback != nil {
		pb := spec.Playback
		ops["play"] = Op{
			Description: "按 url 或 id 解析可播流(视频/直播)",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				return playResolve(ctx, pb, params)
			},
		}
		if pb.GetDanmaku != nil {
			ops["danmaku"] = Op{
				Description: "按 url 或 id 获取弹幕流(subscribe 返回取消函数)",
				Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
					return danmakuResolve(pb, params)
				},
			}
		}
	}
	return ops
}

func fetchChannel(ctx context.Context, byKey map[string]DataChannel, raw json.RawMessage) (any, error) {
	var p fetchParams
	if err := DecodeParams(raw, &p); err != nil {
		return nil, err
	}
	channel, ok := byKey[p.Channel]
	if !ok {
		return nil, fmt.Errorf("未知渠道 %q", p.Channel)
	}
	source := channel.GetSource(p.Info)
	if p.Passive {
		passive, ok := source.(PassiveSource)
		if !ok {
			return nil, fmt.Errorf("渠道 %s 不支持被动采集", channel.Key())
		}
		result, err := passive.FetchPassive(ctx)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("渠道 %s 被动采集无数据", channel.Key())
		}
		return *result, nil
	}
	if p.Cursor != nil {
		pageable, ok := source.(PageableSource)
		if !ok {
			return nil, fmt.Errorf("渠道 %s 不支持翻页", channel.Key())
		}
		existing := -1
		if p.Existing != nil && *p.Existing >= 0 {
			existing = *p.Existing
		}
		return pageable.FetchMore(ctx, *p.Cursor, existing)
	}
	return source.Fetch(ctx)
}

func playTarget(pb *Playback, p playParams) (string, string, error) {
	id := strings.TrimSpace(p.ID)
	kind := p.Kind
	if id == "" && p.URL != "" {
		routeID, routeKind, ok := MatchRoute(pb.Routes, p.URL)
		if !ok {
			return "", "", fmt.Errorf("无法从 URL 解析播放路由: %s", p.URL)
		}
		id, kind = routeID, routeKind
	}
	if id == "" {
		return "", "", fmt.Errorf(".play 需要 url 或 id")
	}
	if kind == "" {
		if pb.ResolveVideoPlay != nil {
			kind = "video"
		} else {
			kind = "live"
		}
	}
	return id, kind, nil
}

func playResolve(ctx context.Context, pb *Playback, raw json.RawMessage) (any, error) {
	var p playParams
	if err := DecodeParams(raw, &p); err != nil {
		return nil, err
	}
	id, kind, err := playTarget(pb, p)
	if err != nil {
		return nil, err
	}
	if kind == "live" {
		if pb.ResolveLivePlay == nil {
			return nil, fmt.Errorf("未实现直播播放解析")
		}
		streams, err := pb.ResolveLivePlay(ctx, id, p.Info)
		if err != nil {
			return nil, err
		}
		return PlayOpResult{ID: id, Streams: streams}, nil
	}
	if pb.ResolveVideoPlay == nil {
		return nil, fmt.Errorf("未实现视频播放解析")
	}
	result, err := pb.ResolveVideoPlay(ctx, id, p.Info)
	if err != nil {
		return nil, err
	}
	return PlayOpResult{ID: id, Streams: result.Streams, Chapters: result.Chapters}, nil
}

// danmakuTarget id-only 时不硬填 kind(交给平台缺省);url 命中时按路由 kind 推断。
func danmakuTarget(pb *Playback, p danmakuParams) (string, string, error) {
	id := strings.TrimSpace(p.ID)
	kind := p.Kind
	if id == "" && p.URL != "" {
		routeID, routeKind, ok := MatchRoute(pb.Routes, p.URL)
		if !ok {
			return "", "", fmt.Errorf("无法从 URL 解析弹幕路由: %s", p.URL)
		}
		id = routeID
		if kind == "" {
			if routeKind == "video" {
				kind = "vod"
			} else {
				kind = "live"
			}
		}
	}
	if id == "" {
		return "", "", fmt.Errorf(".danmaku 需要 url 或 id")
	}
	return id, kind, nil
}

func danmakuResolve(pb *Playback, raw json.RawMessage) (any, error) {
	var p danmakuParams
	if err := DecodeParams(raw, &p); err != nil {
		return nil, err
	}
	if pb.GetDanmaku == nil {
		return nil, fmt.Errorf("未实现弹幕能力")
	}
	id, kind, err := danmakuTarget(pb, p)
	if err != nil {
		return nil, err
	}
	info := DanmakuInfo{Cookie: p.Cookie, Kind: kind}
	return DanmakuOpResult{ID: id, Kind: kind, Subscribe: pb.GetDanmaku(id, info)}, nil
}

func nowReal() int64 { return time.Now().UnixMilli() }
