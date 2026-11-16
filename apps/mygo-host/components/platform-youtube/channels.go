// 渠道清单与单页渠道(youtube:playlist / youtube:user 等):声明式 MakeChannel
// 装配;未声明 Pageable 的渠道由运行时保证不实现翻页(「不支持翻页」语义)。

package youtube

import (
	"context"
	"errors"
	"strings"

	"mygo-host/platform"
)

// channels youtube 平台全部渠道实例(顺序与 TS register.ts 一致)。
func channels(sc *platform.ServiceContext) []platform.DataChannel {
	return []platform.DataChannel{
		newYoutubeChannel(sc),
		newPlaylistChannel(sc),
		newLiveChannel(sc),
		newLiveHotChannel(sc),
		newLiveCategoryChannel(sc),
		newLiveCategoriesChannel(sc),
		newUserChannel(sc),
	}
}

// singlePage 声明式单页渠道(无翻页能力)。
func singlePage(spec platform.ChannelSpec) platform.DataChannel {
	return platform.MakeChannel(spec)
}

func newPlaylistChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:playlist", Name: "YouTube 播放列表", Kind: platform.KindVideo,
		InfoTpl: []platform.SourceInfoField{
			{Key: "playlistId", Label: "播放列表 ID 或链接", Required: platform.Bool(true)},
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			playlistID := normalizeYoutubePlaylistID(info["playlistId"])
			if playlistID == "" {
				return platform.PageItems{}, errors.New("youtube:playlist 需要播放列表 ID 或链接（list=…；不支持 RD/WL/LL 这类动态或私有列表）")
			}
			xmlText, err := sc.HTTP.GetText(ctx,
				"https://www.youtube.com/feeds/videos.xml?playlist_id="+encodeURIComponent(playlistID),
				map[string]string{"user-agent": desktopChromeUA})
			if err != nil {
				return platform.PageItems{}, err
			}
			feed, err := parseFeed(xmlText)
			if err != nil {
				return platform.PageItems{}, err
			}
			now := sc.NowMs()
			items := make([]platform.Item, 0, len(feed.Channel.Items))
			for _, entry := range feed.Channel.Items {
				items = append(items, entryToVideo(entry, now, "youtube:playlist"))
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

func newUserChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:user", Name: "YouTube 搜频道", Kind: platform.KindVideo,
		InfoTpl: []platform.SourceInfoField{
			{Key: "keyword", Label: "频道关键词", Placeholder: platform.Str("如:MKBHD"), Required: platform.Bool(true)},
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			keyword := strings.TrimSpace(info["keyword"])
			if keyword == "" {
				return platform.PageItems{}, errors.New("youtube:user 需要 keyword 参数")
			}
			hits, err := searchYouTubeChannels(ctx, sc, keyword)
			if err != nil {
				return platform.PageItems{}, err
			}
			now := sc.NowMs()
			items := make([]platform.Item, 0, len(hits))
			for _, hit := range hits {
				summary := joinNonEmpty([]string{hit.SubsText, hit.VideoCountText}, " · ")
				items = append(items, platform.Item{
					ID:        "ch-" + hit.ChannelID,
					SourceID:  "youtube:user",
					Kind:      platform.KindVideo,
					Title:     hit.Title,
					URL:       platform.Str("https://www.youtube.com/channel/" + hit.ChannelID),
					Thumbnail: platform.StrOmit(hit.Thumb),
					Summary:   platform.StrOmit(summary),
					FetchedAt: now,
				})
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}
