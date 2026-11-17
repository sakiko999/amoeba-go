// twitch:vod 渠道:某频道的过往直播列表(GQL 游标翻页,复用 cursorSource)。
// login 参数支持裸 login / @login / twitch.tv/xxx 链接(复用 normalizeTwitchLogin)。

package twitch

import (
	"context"
	"fmt"
	"strings"

	"mygo-host/platform"
)

// vodPageSize 列表页大小(videos 连接上限 100,对齐 streams 的 30 档)。
const vodPageSize = 30

// vodChannel twitch:vod —— 频道回放订阅。
type vodChannel struct {
	sc *platform.ServiceContext
}

func (c *vodChannel) Key() string                      { return "twitch:vod" }
func (c *vodChannel) Name() string                     { return "Twitch 直播回放" }
func (c *vodChannel) Kind() platform.Kind              { return platform.KindVideo }
func (c *vodChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *vodChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{{
		Key:         "login",
		Label:       "Twitch 频道名（login 或 twitch.tv/xxx 链接）",
		Required:    platform.Bool(true),
		Placeholder: platform.Str("shroud 或 twitch.tv/shroud"),
	}}
}

func (c *vodChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &cursorSource{
		sc: c.sc, sourceID: c.Key(),
		fetchPage: func(ctx context.Context, after string) (cursorPage, error) {
			return fetchVODItemsPage(ctx, c.sc, info["login"], after)
		},
	}
}

// fetchVODItemsPage login 归一校验 + 列表页 → 已映射游标页。
func fetchVODItemsPage(ctx context.Context, sc *platform.ServiceContext, rawLogin, after string) (cursorPage, error) {
	login, ok := normalizeTwitchLogin(rawLogin)
	if !ok {
		shown := strings.TrimSpace(rawLogin)
		if shown == "" {
			return nil, fmt.Errorf("twitch:vod 需要 login 参数")
		}
		return nil, fmt.Errorf("twitch: 无效频道名 %s", shown)
	}
	page, err := fetchTwitchVODs(ctx, sc, login, vodPageSize, after)
	if err != nil {
		return nil, err
	}
	return vodItemsPage(page, sc.NowMs()), nil
}
