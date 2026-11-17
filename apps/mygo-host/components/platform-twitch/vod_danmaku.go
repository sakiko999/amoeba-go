// twitch VOD 弹幕:VideoCommentsByOffsetOrCursor persisted query 分页拉全量
// (首页按偏移 0,后续按 cursor;cursor 翻页必须移动端 Client-ID)。
// contentOffsetSeconds 即相对回放起点的秒数,×1000 = timeMs。
// 依据 docs/twitch-vod-research.md §4。

package twitch

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mygo-host/platform"
)

const videoCommentsHash = "b70a3591ff0f4e0313d126c6a1502d79a1c02baebb288227c582044aa76adf6a"

var colorHexRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type gqlCommentMessage struct {
	Fragments []struct {
		Text *string `json:"text"`
	} `json:"fragments"`
	UserColor *string `json:"userColor"`
}

type gqlCommentNode struct {
	Commenter *struct {
		Login       *string `json:"login"`
		DisplayName *string `json:"displayName"`
	} `json:"commenter"`
	ContentOffsetSeconds *float64          `json:"contentOffsetSeconds"`
	Message              *gqlCommentMessage `json:"message"`
}

type gqlCommentsPage struct {
	Edges []struct {
		Cursor *string        `json:"cursor"`
		Node   *gqlCommentNode `json:"node"`
	} `json:"edges"`
	PageInfo *gqlPageInfo `json:"pageInfo"`
}

// fetchVideoComments 一页评论(cursor 空 = 首页按偏移 0)。comments 为 null
// (聊天关闭/无权限)按空页处理,video 缺失才报错。
func fetchVideoComments(ctx context.Context, sc *platform.ServiceContext, videoID, cursor string) (*gqlCommentsPage, error) {
	variables := map[string]any{"videoID": videoID, "contentOffsetSeconds": 0}
	if cursor != "" {
		variables = map[string]any{"videoID": videoID, "cursor": cursor}
	}
	body := map[string]any{
		"operationName": "VideoCommentsByOffsetOrCursor",
		"variables":     variables,
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"version": 1, "sha256Hash": videoCommentsHash},
		},
	}
	data, err := twitchGQL[struct {
		Video *struct {
			Comments *gqlCommentsPage `json:"comments"`
		} `json:"video"`
	}](ctx, sc, body, gqlOptions{throwOnErrors: true, clientID: mobileClientID})
	if err != nil {
		return nil, err
	}
	if data.Video == nil {
		return nil, fmt.Errorf("twitch gql: 回放 %s 不存在", videoID)
	}
	if data.Video.Comments == nil {
		return &gqlCommentsPage{}, nil
	}
	return data.Video.Comments, nil
}

// mapComments 一页评论 → DanmakuItem;返回下一个游标(最后一个非空 edge cursor)。
func mapComments(page *gqlCommentsPage) ([]platform.DanmakuItem, string) {
	items := make([]platform.DanmakuItem, 0, len(page.Edges))
	cursor := ""
	for _, edge := range page.Edges {
		if edge.Cursor != nil && *edge.Cursor != "" {
			cursor = *edge.Cursor
		}
		if edge.Node == nil {
			continue
		}
		if item, ok := mapComment(*edge.Node); ok {
			items = append(items, item)
		}
	}
	return items, cursor
}

func mapComment(node gqlCommentNode) (platform.DanmakuItem, bool) {
	text := ""
	if node.Message != nil {
		for _, frag := range node.Message.Fragments {
			text += deref(frag.Text)
		}
	}
	if strings.TrimSpace(text) == "" {
		return platform.DanmakuItem{}, false
	}
	item := platform.DanmakuItem{Text: text}
	if node.Commenter != nil {
		name := strings.TrimSpace(deref(node.Commenter.DisplayName))
		if name == "" {
			name = deref(node.Commenter.Login)
		}
		if name != "" {
			item.User = platform.Str(name)
		}
	}
	if node.Message != nil {
		if color := deref(node.Message.UserColor); colorHexRe.MatchString(color) {
			item.Color = platform.Str(strings.ToUpper(color))
		}
	}
	if node.ContentOffsetSeconds != nil {
		ms := int64(*node.ContentOffsetSeconds * 1000)
		item.TimeMs = &ms
	}
	return item, true
}

// vodDanmakuStream 订阅即后台串行分页拉全量评论(逐页推送);退订停止。
// 游标为空或不前进(上游重复发同一 cursor)、hasNextPage=false 终止,防死循环。
func vodDanmakuStream(sc *platform.ServiceContext, videoID string) platform.DanmakuStream {
	return func(onItems func([]platform.DanmakuItem)) func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			cursor := ""
			for attempts := 0; ; attempts++ {
				if ctx.Err() != nil {
					return
				}
				page, err := fetchVideoComments(ctx, sc, videoID, cursor)
				if err != nil {
					sc.Log("twitch: VOD 弹幕第 %d 页失败: %v", attempts+1, err)
					return
				}
				items, next := mapComments(page)
				if len(items) > 0 {
					onItems(items)
				}
				if next == "" || next == cursor || !commentsHasMore(page) {
					return
				}
				cursor = next
			}
		}()
		return cancel
	}
}

func commentsHasMore(page *gqlCommentsPage) bool {
	return page.PageInfo != nil && page.PageInfo.HasNextPage != nil && *page.PageInfo.HasNextPage
}

// unimplementedLiveDanmaku 直播弹幕(IRC over WS)未实现:订阅即日志说明,静默无数据。
func unimplementedLiveDanmaku(sc *platform.ServiceContext) platform.DanmakuStream {
	return func(func([]platform.DanmakuItem)) func() {
		sc.Log("twitch: 直播弹幕未实现(IRC 二期),本次订阅无数据")
		return func() {}
	}
}
