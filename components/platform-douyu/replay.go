// douyu 回放数据面(调研:docs/douyu-replay-research.md,2026-11-16 实测):
// betard 取 up_id → authorShowVideoList 列场次(re_num>1 需 getShowReplayList
// 展开分P)→ 详情页正则取 point_id → 复用直播签名链(homeH5Enc+crptext,
// 第一参数换 point_id)→ getStreamUrlWeb 取 HLS VOD(短时 token,勿持久化)。

package douyu

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// vodBaseURL 回放域名;var 供离线测试覆写。
var vodBaseURL = "https://v.douyu.com"

var pointIDRe = regexp.MustCompile(`"point_id":(\d+)`)

const replayPageSize = 20

// vodGetJSON 带 UA/referer 的 GET JSON(v.douyu.com 系端点)。
func vodGetJSON(ctx context.Context, sc *platform.ServiceContext, url string, out any) error {
	raw, err := sc.HTTP.GetJSON(ctx, url, map[string]string{
		"user-agent": clientUA,
		"referer":    vodBaseURL + "/",
	})
	if err != nil {
		return err
	}
	return platform.UnmarshalJSON(raw, out)
}

// fetchUpID 房间号 → 主播 up_id(回放按主播组织,只认哈希形态)。
func fetchUpID(ctx context.Context, sc *platform.ServiceContext, roomID string) (string, error) {
	room, err := roomInfo(ctx, sc, roomID)
	if err != nil {
		return "", err
	}
	upID := platform.JSString(room.UpID)
	if upID == "" {
		return "", fmt.Errorf("douyu: 房间 %s 无 up_id,无法列回放", roomID)
	}
	return upID, nil
}

// fetchReplayShows 按主播列回放场次;count 为场次总数(跨页)。
func fetchReplayShows(ctx context.Context, sc *platform.ServiceContext, upID string, page int) ([]map[string]any, int, error) {
	url := fmt.Sprintf("%s/wgapi/vod/center/authorShowVideoList?up_id=%s&page=%d&limit=%d",
		vodBaseURL, upID, page, replayPageSize)
	var res replayListResponse
	if err := vodGetJSON(ctx, sc, url, &res); err != nil {
		return nil, 0, err
	}
	if n, ok := platform.ToInt(res.Error); ok && n != 0 {
		return nil, 0, fmt.Errorf("douyu: 回放列表 error %s: %s", platform.JSString(res.Error), platform.JSString(res.Msg))
	}
	count, _ := platform.ToInt(res.Data.Count)
	return res.Data.List, int(count), nil
}

// fetchShowParts 展开多分P场次(2h+ 回放被切成 ~2h 分P,列表只带第 1 个)。
func fetchShowParts(ctx context.Context, sc *platform.ServiceContext, firstHashID, upID string) ([]map[string]any, error) {
	url := fmt.Sprintf("%s/wgapi/vod/center/getShowReplayList?vid=%s&up_id=%s", vodBaseURL, firstHashID, upID)
	var res replayPartsResponse
	if err := vodGetJSON(ctx, sc, url, &res); err != nil {
		return nil, err
	}
	if len(res.Data.List) == 0 {
		return nil, fmt.Errorf("douyu: 分P展开为空(%s)", firstHashID)
	}
	return res.Data.List, nil
}

// showPointID 详情页 HTML 的 $DATA 里提取 point_id(签名用数字 id,无 JSON 接口)。
func showPointID(ctx context.Context, sc *platform.ServiceContext, hashID string) (string, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:     vodBaseURL + "/show/" + hashID,
		Headers: map[string]string{"user-agent": clientUA},
	})
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", fmt.Errorf("douyu HTTP %d: show/%s", res.Status, hashID)
	}
	match := pointIDRe.FindStringSubmatch(res.Body)
	if match == nil {
		return "", fmt.Errorf("douyu: show/%s 未找到 point_id", hashID)
	}
	return match[1], nil
}

// vodStreamData POST getStreamUrlWeb(签名体 + vid=hash_id)返回 data。
func vodStreamData(ctx context.Context, sc *platform.ServiceContext, signed, hashID string) (map[string]map[string]any, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:    vodBaseURL + "/wgapi/vodnc/front/stream/getStreamUrlWeb",
		Method: "POST",
		Headers: map[string]string{
			"content-type": "application/x-www-form-urlencoded",
			"user-agent":   clientUA,
			"referer":      vodBaseURL + "/show/" + hashID,
		},
		Body: signed + "&vid=" + hashID,
	})
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("douyu HTTP %d: getStreamUrlWeb", res.Status)
	}
	var payload vodStreamResponse
	if err := json.Unmarshal([]byte(res.Body), &payload); err != nil {
		return nil, err
	}
	if n, ok := platform.ToInt(payload.Error); ok && n != 0 {
		return nil, fmt.Errorf("douyu: 回放取流失败(%s): %s", platform.JSString(payload.Error), platform.JSString(payload.Msg))
	}
	return payload.Data.ThumbVideo, nil
}

// thumbVideoStreams 档位 key→条目 映射 → 档位列表(按 bit_rate 降序;key 动态,
// 接口顺序不保证高档在前)。URL 统一升 https(防 renderer 混合内容)。
func thumbVideoStreams(thumb map[string]map[string]any, hashID string) []platform.Stream {
	type entry struct {
		quality string
		rate    float64
		url     string
	}
	list := make([]entry, 0, len(thumb))
	for _, item := range thumb {
		url := platform.JSString(item["url"])
		if url == "" {
			continue
		}
		rate, _ := platform.ToInt(item["bit_rate"])
		list = append(list, entry{quality: platform.JSString(item["name"]), rate: rate, url: url})
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].rate > list[j].rate })
	streams := make([]platform.Stream, 0, len(list))
	for _, e := range list {
		streams = append(streams, platform.Stream{
			URL:     strings.Replace(e.url, "http://", "https://", 1),
			Format:  platform.Str("hls"),
			Quality: platform.Str(e.quality),
			Headers: map[string]string{"referer": vodBaseURL + "/show/" + hashID, "user-agent": clientUA},
		})
	}
	if len(streams) == 0 {
		return nil
	}
	return streams
}

// resolveVideoStreams 回放可播流:point_id → 签名(参数从 roomId 换 point_id)→
// getStreamUrlWeb。Stream.url 带短时 token(tlink≈2h/tplay≈12h),消费方不得持久化。
func resolveVideoStreams(ctx context.Context, sc *platform.ServiceContext, hashID string) ([]platform.Stream, error) {
	pointID, err := showPointID(ctx, sc, hashID)
	if err != nil {
		return nil, err
	}
	crptext, err := signPayload(ctx, sc, pointID)
	if err != nil {
		return nil, err
	}
	signed, err := sign(ctx, sc, crptext, pointID)
	if err != nil {
		return nil, err
	}
	thumb, err := vodStreamData(ctx, sc, signed, hashID)
	if err != nil {
		return nil, err
	}
	streams := thumbVideoStreams(thumb, hashID)
	if len(streams) == 0 {
		return nil, fmt.Errorf("douyu: 回放 %s 无可播档位", hashID)
	}
	return streams, nil
}

// parseVideoDuration "113:21" / "02:00:02" / "16:47"(从右往左 秒:分:时)→ 秒。
// video_duration 数字字段实测恒为 0,一律用格式化串。
func parseVideoDuration(s string) *float64 {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) == 0 || len(parts) > 3 {
		return nil
	}
	total := 0.0
	mult := 1.0
	for i := len(parts) - 1; i >= 0; i-- {
		n, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		if err != nil || n < 0 {
			return nil
		}
		total += n * mult
		mult *= 60
	}
	if total == 0 {
		return nil
	}
	return &total
}
