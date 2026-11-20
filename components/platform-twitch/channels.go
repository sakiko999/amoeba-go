// twitch 渠道实现:直播订阅(roomIds)、回放(vodChannel,Go 侧新增)、热门/分区
// (游标翻页)、分区列表与频道搜索(声明式单页渠道)。除 twitch:vod 外,
// 渠道 key/名称/kind/参数表与 TS 完全一致。

package twitch

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mygo-host/platform"
)

var roomSeparator = regexp.MustCompile(`[,，\s]+`)

// parseRoomIds roomIds 逗号/空白分隔(空则 []);兼容中文逗号。
func parseRoomIds(info platform.SourceInfo) []string {
	parts := roomSeparator.Split(info["roomIds"], -1)
	rooms := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			rooms = append(rooms, part)
		}
	}
	return rooms
}

// liveChannel twitch:live —— 频道订阅(别名批量查询,离线仍产出)。
type liveChannel struct {
	sc *platform.ServiceContext
}

func (c *liveChannel) Key() string                      { return "twitch:live" }
func (c *liveChannel) Name() string                     { return "Twitch 直播" }
func (c *liveChannel) Kind() platform.Kind              { return platform.KindLive }
func (c *liveChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *liveChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{{
		Key:         "roomIds",
		Label:       "Twitch 频道名（逗号分隔，可多个）",
		Required:    platform.Bool(true),
		Placeholder: platform.Str("shroud,riotgames 或 twitch.tv/xxx"),
	}}
}

func (c *liveChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &liveSource{sc: c.sc, info: info}
}

type liveSource struct {
	sc   *platform.ServiceContext
	info platform.SourceInfo
}

func (s *liveSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	raw := parseRoomIds(s.info)
	if len(raw) == 0 {
		return platform.FetchResult{}, fmt.Errorf("twitch:live 需要 roomIds")
	}
	logins := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, value := range raw {
		login, ok := normalizeTwitchLogin(value)
		if !ok || seen[login] {
			continue
		}
		seen[login] = true
		logins = append(logins, login)
	}
	if len(logins) == 0 {
		return platform.FetchResult{}, fmt.Errorf("twitch:live 没有合法频道名(%d 个)", len(raw))
	}
	fetchedAt := s.sc.NowMs()
	users, err := fetchTwitchUsers(ctx, s.sc, logins)
	if err != nil {
		return platform.FetchResult{}, err
	}
	items := make([]platform.Item, 0, len(logins))
	for i, login := range logins {
		if i >= len(users) || users[i] == nil {
			s.sc.Log("twitch:live 频道 %s 不存在,跳过", login)
			continue
		}
		items = append(items, twitchLiveItem(users[i], login, fetchedAt))
	}
	if len(items) == 0 {
		return platform.FetchResult{}, fmt.Errorf("twitch:live 全部频道拉取失败(%d 个)", len(logins))
	}
	return platform.FetchResult{Items: items}, nil
}

// hotChannel twitch:hot —— 热门直播(无参数,游标翻页)。
type hotChannel struct {
	sc *platform.ServiceContext
}

func (c *hotChannel) Key() string         { return "twitch:hot" }
func (c *hotChannel) Name() string        { return "Twitch 热门直播" }
func (c *hotChannel) Kind() platform.Kind { return platform.KindLive }

// SourceInfoTpl TS 声明为空数组(channels op 输出 "sourceInfoTpl": []),非 nil。
func (c *hotChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{}
}

func (c *hotChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *hotChannel) GetSource(platform.SourceInfo) platform.DataSource {
	return &cursorSource{
		sc: c.sc, sourceID: c.Key(),
		fetchPage: func(ctx context.Context, after string) (cursorPage, error) {
			page, err := fetchTwitchTopStreams(ctx, c.sc, 30, after)
			if err != nil {
				return nil, err
			}
			return streamItemsPage(page, c.Key(), c.sc.NowMs()), nil
		},
	}
}

// categoryChannel twitch:category —— 分区直播(参数 category 可裸 slug 或目录链接)。
type categoryChannel struct {
	sc *platform.ServiceContext
}

func (c *categoryChannel) Key() string                      { return "twitch:category" }
func (c *categoryChannel) Name() string                     { return "Twitch 分区直播" }
func (c *categoryChannel) Kind() platform.Kind              { return platform.KindLive }
func (c *categoryChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *categoryChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{{
		Key:         "category",
		Label:       "分类 slug 或目录链接",
		Required:    platform.Bool(true),
		Placeholder: platform.Str("just-chatting 或 twitch.tv/directory/category/just-chatting"),
	}}
}

func (c *categoryChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	raw := info["category"]
	return &cursorSource{
		sc: c.sc, sourceID: c.Key(),
		fetchPage: func(ctx context.Context, after string) (cursorPage, error) {
			slug, ok := normalizeTwitchCategory(raw)
			if !ok {
				shown := strings.TrimSpace(raw)
				if shown == "" {
					shown = "空"
				}
				return nil, fmt.Errorf("twitch:category 分类无效(%s)", shown)
			}
			page, err := fetchTwitchCategoryStreams(ctx, c.sc, slug, 30, after)
			if err != nil {
				return nil, err
			}
			return streamItemsPage(page, c.Key(), c.sc.NowMs()), nil
		},
	}
}

// newCategoriesChannel twitch:categories —— 分类目录订阅(声明式单页渠道)。
func newCategoriesChannel(sc *platform.ServiceContext) platform.DataChannel {
	inner := platform.MakeChannel(platform.ChannelSpec{
		Key:         "twitch:categories",
		Name:        "Twitch 分区列表",
		Kind:        platform.KindCategory,
		DefaultInfo: platform.SourceInfo{},
		FetchPage: func(ctx context.Context, _ platform.SourceInfo, _ int) (platform.PageItems, error) {
			games, err := fetchTwitchTopCategories(ctx, sc, 30)
			if err != nil {
				return platform.PageItems{}, err
			}
			fetchedAt := sc.NowMs()
			items := make([]platform.Item, 0, len(games))
			for _, game := range games {
				items = append(items, categoryItem(game, fetchedAt))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
	return inner
}

// newUserSearchEntity twitch:user:search —— 按关键词搜频道(声明式单页渠道)。
func newUserSearchChannel(sc *platform.ServiceContext) platform.DataChannel {
	inner := platform.MakeChannel(platform.ChannelSpec{
		Key:  "twitch:user:search",
		Name: "Twitch 搜频道",
		Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{{
			Key:         "keyword",
			Label:       "主播/频道关键词",
			Required:    platform.Bool(true),
			Placeholder: platform.Str("如:shroud"),
		}},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, _ int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["keyword"])
			if keyword == "" {
				return platform.PageItems{}, fmt.Errorf("twitch:user:search 需要 keyword 参数")
			}
			hits, err := searchTwitchChannels(ctx, sc, keyword)
			if err != nil {
				return platform.PageItems{}, err
			}
			fetchedAt := sc.NowMs()
			items := make([]platform.Item, 0, len(hits))
			for _, hit := range hits {
				items = append(items, searchItem(hit, fetchedAt))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
	return inner
}
