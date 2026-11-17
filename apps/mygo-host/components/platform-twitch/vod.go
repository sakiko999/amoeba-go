// twitch VOD(过往直播)列表与映射:user(login).videos 连接(游标翻页,
// after 必须走移动端 Client-ID);RECORDING 场次长度仍在增长,标记不丢弃。
// 依据 docs/twitch-vod-research.md §2。

package twitch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mygo-host/platform"
)

const vodFields = `id title lengthSeconds createdAt publishedAt viewCount status broadcastType previewThumbnailURL(width: 640, height: 360) owner { login displayName } game { name }`

// twitchVODPage 一页 VOD;EndCursor 取本页最后一个非空 edge cursor(同页 cursor 可能全部相同)。
type twitchVODPage struct {
	VODs      []gqlVideoNode
	Total     *int
	HasMore   bool
	EndCursor *string
}

// fetchTwitchVODs 某频道过往直播(type=ARCHIVE,按时间倒序)。first 上限 100。
func fetchTwitchVODs(ctx context.Context, sc *platform.ServiceContext, login string, first int, after string) (*twitchVODPage, error) {
	args := []string{fmt.Sprintf("first: %d", clampVODFirst(first)), "type: ARCHIVE", "sort: TIME"}
	if after != "" {
		args = append(args, "after: "+jsonString(after))
	}
	query := fmt.Sprintf("query { user(login: %s) { videos(%s) { totalCount edges { cursor node { %s } } pageInfo { hasNextPage } } } }",
		jsonString(login), strings.Join(args, ", "), vodFields)
	data, err := twitchGQL[struct {
		User *struct {
			Videos *gqlVideoConnection `json:"videos"`
		} `json:"user"`
	}](ctx, sc, map[string]any{"query": query},
		gqlOptions{throwOnErrors: true, clientID: mobileClientID})
	if err != nil {
		return nil, err
	}
	if data.User == nil || data.User.Videos == nil {
		return nil, fmt.Errorf("twitch gql: 频道 %s 不存在或无回放列表", login)
	}
	return mapVODPage(data.User.Videos), nil
}

func mapVODPage(conn *gqlVideoConnection) *twitchVODPage {
	page := &twitchVODPage{Total: conn.TotalCount, VODs: make([]gqlVideoNode, 0, len(conn.Edges))}
	for _, edge := range conn.Edges {
		if edge.Node != nil {
			page.VODs = append(page.VODs, *edge.Node)
		}
		if edge.Cursor != nil && *edge.Cursor != "" {
			page.EndCursor = edge.Cursor
		}
	}
	if conn.PageInfo != nil && conn.PageInfo.HasNextPage != nil {
		page.HasMore = *conn.PageInfo.HasNextPage
	}
	return page
}

// vodItemsPage VOD 页 → 已映射游标页。
func vodItemsPage(page *twitchVODPage, fetchedAt int64) *twitchItemsPage {
	items := make([]platform.Item, 0, len(page.VODs))
	for _, node := range page.VODs {
		if item := vodItem(node, fetchedAt); item != nil {
			items = append(items, *item)
		}
	}
	return &twitchItemsPage{Items: items, Total: page.Total, HasMore: page.HasMore, EndCursor: page.EndCursor}
}

// vodItem VOD 条目 → Video(stream 留空:VOD token 短时,play 懒解析)。
func vodItem(node gqlVideoNode, fetchedAt int64) *platform.Item {
	id := deref(node.ID)
	if id == "" {
		return nil
	}
	title := strings.TrimSpace(deref(node.Title))
	if title == "" {
		title = "未命名回放"
	}
	item := platform.Item{
		ID:        "twitch:vod:" + id,
		SourceID:  "twitch:vod",
		Kind:      platform.KindVideo,
		Title:     title,
		FetchedAt: fetchedAt,
		URL:       platform.Str("https://www.twitch.tv/videos/" + id),
		Thumbnail: platform.StrOmit(deref(node.PreviewThumbnailURL)),
		Platform:  "twitch",
	}
	if node.LengthSeconds != nil {
		item.Duration = platform.Float64(float64(*node.LengthSeconds))
	}
	if ms, ok := rfc3339ToMs(deref(node.CreatedAt)); ok {
		item.PublishedAt = &ms
	}
	if node.Owner != nil {
		if name := strings.TrimSpace(deref(node.Owner.DisplayName)); name != "" {
			item.Author = &platform.Author{Name: name}
		}
	}
	if summary := vodSummary(node); summary != "" {
		item.Summary = platform.Str(summary)
	}
	return &item
}

// vodSummary 次行:RECORDING 标记录中(长度持续增长);观看数缩写。
func vodSummary(node gqlVideoNode) string {
	parts := make([]string, 0, 2)
	if strings.EqualFold(deref(node.Status), "RECORDING") {
		parts = append(parts, "录制中")
	}
	if node.ViewCount != nil && *node.ViewCount > 0 {
		parts = append(parts, fmtCount(*node.ViewCount)+" 次观看")
	}
	return strings.Join(parts, " · ")
}

func rfc3339ToMs(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0, false
	}
	return t.UnixMilli(), true
}

// clampVODFirst videos 连接的 first 上限 100(streams 是 30,故单独钳制)。
func clampVODFirst(first int) int {
	if first < 1 {
		return 1
	}
	if first > 100 {
		return 100
	}
	return first
}
