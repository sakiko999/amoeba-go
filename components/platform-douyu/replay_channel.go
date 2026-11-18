// douyu:replay 渠道:单房间维度翻页列主播回放场次;多分P场次展开为
// 每分P一条 item(2h+ 回放列表只带第 1 个分P,缺展开会缺后半段)。

package douyu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mygo-host/platform"
)

// newReplayChannel 斗鱼直播回放(roomId → up_id → 场次分页)。
func newReplayChannel(sc *platform.ServiceContext) platform.DataChannel {
	return platform.MakeChannel(platform.ChannelSpec{
		Key:  "douyu:replay",
		Name: "斗鱼直播回放",
		Kind: platform.KindVideo,
		InfoTpl: []platform.SourceInfoField{{
			Key: "roomId", Label: "直播间 ID", Required: platform.Bool(true),
		}},
		Pageable: &platform.PageableSpec{First: 2, Step: 1},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			roomID := strings.TrimSpace(platform.JSString(info["roomId"]))
			if roomID == "" {
				return platform.PageItems{}, errors.New("douyu:replay 需要 roomId 参数")
			}
			upID, err := fetchUpID(ctx, sc, roomID)
			if err != nil {
				return platform.PageItems{}, err
			}
			shows, count, err := fetchReplayShows(ctx, sc, upID, page)
			if err != nil {
				return platform.PageItems{}, err
			}
			items, err := expandReplayShows(ctx, sc, shows, upID)
			if err != nil {
				return platform.PageItems{}, err
			}
			return platform.PageItems{
				Items:   items,
				HasMore: platform.Bool(page*replayPageSize < count),
			}, nil
		},
	})
}

// expandReplayShows 场次 → item 列表:re_num>1 的场次经分P接口展开(失败降级
// 为仅第 1 分P并记日志——部分可用优于整页失败);单分P场次直接用 video_list。
func expandReplayShows(ctx context.Context, sc *platform.ServiceContext, shows []map[string]any, upID string) ([]platform.Item, error) {
	items := make([]platform.Item, 0, len(shows))
	t := sc.NowMs()
	for _, show := range shows {
		parts := showPartList(show)
		reNum, _ := platform.ToInt(show["re_num"])
		if reNum > 1 && len(parts) > 0 {
			expanded, err := fetchShowParts(ctx, sc, platform.JSString(parts[0]["hash_id"]), upID)
			if err != nil {
				sc.Log("douyu: 分P展开失败,降级仅第 1 分P: %v", err)
			} else {
				parts = expanded
			}
		}
		for i, part := range parts {
			items = append(items, replayItem(show, part, i, len(parts), t))
		}
	}
	if len(items) == 0 && len(shows) > 0 {
		return nil, errors.New("douyu: 回放场次无可映射条目")
	}
	return items, nil
}

// showPartList 场次的 video_list 条目(列表只含第 1 个分P)。
func showPartList(show map[string]any) []map[string]any {
	arr, _ := show["video_list"].([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, entry := range arr {
		if m, ok := entry.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// replayItem 场次 + 分P → item。列表条目时长在 video_str_duration,分P条目在
// video_duration(两者都是格式化串);多分P时标题带 Pn/N 后缀区分。
func replayItem(show, part map[string]any, index, total int, t int64) platform.Item {
	hashID := platform.JSString(part["hash_id"])
	title := htmlUnescape(platform.JSString(show["title"]))
	if total > 1 {
		title = fmt.Sprintf("%s(P%d/%d)", title, index+1, total)
	}
	duration := parseVideoDuration(platform.JSString(part["video_str_duration"]))
	if duration == nil {
		duration = parseVideoDuration(platform.JSString(part["video_duration"]))
	}
	item := platform.Item{
		ID:        "douyu:vod:" + hashID,
		SourceID:  "douyu:replay",
		Kind:      platform.KindVideo,
		Title:     title,
		URL:       platform.Str(vodBaseURL + "/show/" + hashID),
		FetchedAt: t,
		Platform:  "douyu",
		Duration:  duration,
		// stream 留空:VOD 直链带短时 token,play 时懒解析。
	}
	if start, ok := platform.ToInt(show["start_time"]); ok && start > 0 {
		ms := int64(start * 1000)
		item.PublishedAt = &ms
	}
	if raw, err := json.Marshal(part); err == nil {
		item.Raw = platform.RawJSON(raw)
	}
	return item
}
