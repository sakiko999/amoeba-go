// twitch 匿名 GraphQL 客户端:元数据查询与播放 token 走网页 Client-ID;
// 游标翻页(带 after 的流列表)必须用 Android app 的公开 Client-ID,否则第二页起
// 触发 Client-Integrity 校验。错误文案只带 HTTP 状态/上游 message,不含 token/signature。

package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mygo-host/platform"
)

const (
	webClientID    = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	mobileClientID = "kd1unb4b3q4t58fwlpcbzcbnm76a8fp"
	userFields     = `login displayName description stream { id title viewersCount createdAt type game { name } }`
	streamFields   = `id title viewersCount createdAt broadcaster { login displayName } game { name slug } previewImageURL(width: 640, height: 360)`
)

// gqlURL/usherBase 为变量:测试可用本地 httptest 端点替换。
var gqlURL = "https://gql.twitch.tv/gql"

type gqlOptions struct {
	throwOnErrors bool
	clientID      string
}

type gqlErrorItem struct {
	Message *string `json:"message"`
}

// twitchGQL POST 一次 GraphQL,返回 data(无 data 时返回 T 零值)。
func twitchGQL[T any](ctx context.Context, sc *platform.ServiceContext, body any, opts gqlOptions) (T, error) {
	var zero T
	clientID := opts.clientID
	if clientID == "" {
		clientID = webClientID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return zero, err
	}
	text, err := postGQL(ctx, sc, encoded, clientID)
	if err != nil {
		return zero, err
	}
	return decodeGQL[T](text, opts)
}

func postGQL(ctx context.Context, sc *platform.ServiceContext, encoded []byte, clientID string) (string, error) {
	res, err := sc.HTTP.Do(ctx, platform.Request{
		URL:    gqlURL,
		Method: "POST",
		Headers: map[string]string{
			"client-id":    clientID,
			"content-type": "application/json",
		},
		Body: string(encoded),
	})
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", fmt.Errorf("twitch gql HTTP %d", res.Status)
	}
	return res.Body, nil
}

// decodeGQL 解 envelope:顶层 errors 在无 data 或 throwOnErrors 时上抛。
func decodeGQL[T any](text string, opts gqlOptions) (T, error) {
	var zero T
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlErrorItem  `json:"errors"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return zero, fmt.Errorf("twitch gql: 响应不是合法 JSON")
	}
	hasData := len(envelope.Data) > 0 && !bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null"))
	if message := firstErrorMessage(envelope.Errors); message != "" && (!hasData || opts.throwOnErrors) {
		return zero, fmt.Errorf("twitch gql: %s", message)
	}
	if !hasData {
		return zero, nil
	}
	if err := json.Unmarshal(envelope.Data, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func firstErrorMessage(items []gqlErrorItem) string {
	if len(items) == 0 || items[0].Message == nil {
		return ""
	}
	return *items[0].Message
}

// fetchTwitchUsers 批量查频道元数据(GraphQL 别名合并成一次请求);不存在项为 nil。
func fetchTwitchUsers(ctx context.Context, sc *platform.ServiceContext, logins []string) ([]*gqlUser, error) {
	if len(logins) == 0 {
		return []*gqlUser{}, nil
	}
	fields := make([]string, 0, len(logins))
	for i, login := range logins {
		fields = append(fields, fmt.Sprintf("u%d: user(login: %s) { %s }", i, jsonString(login), userFields))
	}
	data, err := twitchGQL[map[string]*gqlUser](ctx, sc,
		map[string]any{"query": "query { " + strings.Join(fields, " ") + " }"}, gqlOptions{})
	if err != nil {
		return nil, err
	}
	users := make([]*gqlUser, len(logins))
	for i := range logins {
		users[i] = data[fmt.Sprintf("u%d", i)]
	}
	return users, nil
}

// twitchStreamPage 一页直播流(endCursor 省略 = 上游未给出)。
type twitchStreamPage struct {
	Streams   []gqlStreamNode
	HasMore   bool
	EndCursor *string
}

// fetchTwitchTopStreams 热门直播(按观看数降序,游标翻页)。
func fetchTwitchTopStreams(ctx context.Context, sc *platform.ServiceContext, first int, after string) (*twitchStreamPage, error) {
	args := []string{fmt.Sprintf("first: %d", clampFirst(first)), "options: { sort: VIEWER_COUNT }"}
	if after != "" {
		args = append(args, "after: "+jsonString(after))
	}
	query := fmt.Sprintf("query { streams(%s) { edges { cursor node { %s } } pageInfo { hasNextPage endCursor } } }",
		strings.Join(args, ", "), streamFields)
	data, err := twitchGQL[struct {
		Streams *gqlStreamConnection `json:"streams"`
	}](ctx, sc, map[string]any{"query": query},
		gqlOptions{throwOnErrors: true, clientID: mobileClientID})
	if err != nil {
		return nil, err
	}
	return mapStreamPage(data.Streams, "twitch gql: 热门直播缺少 streams")
}

// fetchTwitchTopCategories 分类榜(按观看数降序;无 slug 的脏数据丢弃)。
func fetchTwitchTopCategories(ctx context.Context, sc *platform.ServiceContext, first int) ([]gqlCategoryNode, error) {
	query := fmt.Sprintf("query { games(first: %d) { edges { node { id name slug viewersCount boxArtURL(width: 240, height: 320) } } } }",
		clampFirst(first))
	data, err := twitchGQL[struct {
		Games *gqlCategoryConnection `json:"games"`
	}](ctx, sc, map[string]any{"query": query},
		gqlOptions{throwOnErrors: true, clientID: mobileClientID})
	if err != nil {
		return nil, err
	}
	if data.Games == nil {
		return nil, fmt.Errorf("twitch gql: 分类列表缺少 games")
	}
	nodes := make([]gqlCategoryNode, 0, len(data.Games.Edges))
	for _, edge := range data.Games.Edges {
		if edge.Node == nil || edge.Node.Slug == nil || *edge.Node.Slug == "" {
			continue
		}
		nodes = append(nodes, *edge.Node)
	}
	return nodes, nil
}

// twitchChannelHit 频道搜索结果(匿名 searchFor)。
type twitchChannelHit struct {
	Login       string
	DisplayName string
	Description string
	Followers   int
}

func searchTwitchChannels(ctx context.Context, sc *platform.ServiceContext, userQuery string) ([]twitchChannelHit, error) {
	q := strings.TrimSpace(userQuery)
	if q == "" {
		return []twitchChannelHit{}, nil
	}
	body := map[string]any{
		"query": `query Search($q: String!) {
        searchFor(userQuery: $q, platform: "web", target: null) {
          channels { items { login displayName description followers { totalCount } } }
        }
      }`,
		"variables": map[string]any{"q": q},
	}
	data, err := twitchGQL[struct {
		SearchFor *struct {
			Channels *struct {
				Items []gqlChannelHit `json:"items"`
			} `json:"channels"`
		} `json:"searchFor"`
	}](ctx, sc, body, gqlOptions{throwOnErrors: true})
	if err != nil {
		return nil, err
	}
	if data.SearchFor == nil || data.SearchFor.Channels == nil {
		return nil, fmt.Errorf("twitch gql: 搜索缺少 channels")
	}
	hits := make([]twitchChannelHit, 0, len(data.SearchFor.Channels.Items))
	for _, item := range data.SearchFor.Channels.Items {
		if hit := mapChannelHit(item); hit != nil {
			hits = append(hits, *hit)
		}
	}
	return hits, nil
}

func mapChannelHit(item gqlChannelHit) *twitchChannelHit {
	login := strings.ToLower(strings.TrimSpace(deref(item.Login)))
	if login == "" {
		return nil
	}
	displayName := strings.TrimSpace(deref(item.DisplayName))
	if displayName == "" {
		displayName = login
	}
	followers := 0
	if item.Followers != nil && item.Followers.TotalCount != nil {
		followers = *item.Followers.TotalCount
	}
	return &twitchChannelHit{
		Login:       login,
		DisplayName: displayName,
		Description: strings.TrimSpace(deref(item.Description)),
		Followers:   followers,
	}
}

// fetchTwitchCategoryStreams 分区直播(`game(slug).streams`,游标翻页)。
func fetchTwitchCategoryStreams(ctx context.Context, sc *platform.ServiceContext, slug string, first int, after string) (*twitchStreamPage, error) {
	args := []string{fmt.Sprintf("first: %d", clampFirst(first))}
	if after != "" {
		args = append(args, "after: "+jsonString(after))
	}
	query := fmt.Sprintf("query { game(slug: %s) { displayName streams(%s) { edges { cursor node { %s } } pageInfo { hasNextPage endCursor } } } }",
		jsonString(slug), strings.Join(args, ", "), streamFields)
	data, err := twitchGQL[struct {
		Game *struct {
			DisplayName *string              `json:"displayName"`
			Streams     *gqlStreamConnection `json:"streams"`
		} `json:"game"`
	}](ctx, sc, map[string]any{"query": query},
		gqlOptions{throwOnErrors: true, clientID: mobileClientID})
	if err != nil {
		return nil, err
	}
	if data.Game == nil {
		return nil, fmt.Errorf("twitch gql: 分区 %s 不存在", slug)
	}
	return mapStreamPage(data.Game.Streams, fmt.Sprintf("twitch gql: 分区 %s 缺少 streams", slug))
}

// mapStreamPage 一页 GraphQL 流列表 → 归一结构;连接缺失视为 schema 变化并报错。
func mapStreamPage(conn *gqlStreamConnection, missingMessage string) (*twitchStreamPage, error) {
	if conn == nil {
		return nil, fmt.Errorf("%s", missingMessage)
	}
	streams := make([]gqlStreamNode, 0, len(conn.Edges))
	for _, edge := range conn.Edges {
		if edge.Node != nil {
			streams = append(streams, *edge.Node)
		}
	}
	page := &twitchStreamPage{Streams: streams}
	if conn.PageInfo != nil {
		if conn.PageInfo.HasNextPage != nil {
			page.HasMore = *conn.PageInfo.HasNextPage
		}
		if conn.PageInfo.EndCursor != nil && *conn.PageInfo.EndCursor != "" {
			page.EndCursor = conn.PageInfo.EndCursor
		}
	}
	return page, nil
}

// clampFirst 上游硬上限 30;非正数回落到 1。
func clampFirst(first int) int {
	if first < 1 {
		return 1
	}
	if first > 30 {
		return 30
	}
	return first
}

func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
