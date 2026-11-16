// 直播渠道族:youtube:live(videoId 或 channelId 锚点)、youtube:live-hot
// (Live Now 栏目)、youtube:live-category(按栏目)、youtube:live-categories(栏目目录)。
// 直播页解析结果在 discover.go;栏目条目按观看数降序。

package youtube

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mygo-host/platform"
)

var (
	channelIDRe     = regexp.MustCompile(`^UC[\w-]{20,}$`)
	liveNowPrefixRe = regexp.MustCompile(`(?i)^live now\s*-\s*`)
)

func isChannelID(value string) bool { return channelIDRe.MatchString(value) }

func newLiveChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:live", Name: "YouTube 直播", Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{
			{Key: "videoId", Label: "直播或频道 ID", Required: platform.Bool(true)},
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			raw := strings.TrimSpace(info["videoId"])
			if raw == "" {
				return platform.PageItems{}, errors.New("youtube:live 需要 videoId")
			}
			now := sc.NowMs()
			if isChannelID(raw) {
				item, err := channelLiveItem(ctx, sc, raw, now)
				if err != nil {
					return platform.PageItems{}, err
				}
				return platform.PageItems{Items: []platform.Item{item}}, nil
			}
			meta := getYoutubeLiveMeta(ctx, sc, raw)
			return platform.PageItems{Items: []platform.Item{videoLiveItem(raw, now, meta)}}, nil
		},
	})
}

// videoLiveItem videoId 直订:真实元信息,失败回退占位(声明态 live)。
func videoLiveItem(videoID string, now int64, meta *liveMeta) platform.Item {
	title := "YouTube " + videoID
	status := platform.LiveStatusLive
	var author *platform.Author
	var online *int
	if meta != nil {
		if meta.Title != "" {
			title = meta.Title
		}
		if meta.Author != "" {
			author = &platform.Author{Name: meta.Author}
		}
		if meta.IsLive {
			status = platform.LiveStatusLive
		} else {
			status = platform.LiveStatusOffline
		}
		online = meta.Online
	}
	thumb := "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"
	return platform.Item{
		ID:         videoID,
		SourceID:   "youtube:live",
		Kind:       platform.KindLive,
		Title:      title,
		URL:        platform.Str("https://www.youtube.com/watch?v=" + videoID),
		Thumbnail:  platform.Str(thumb),
		Poster:     platform.Str(thumb),
		Author:     author,
		FetchedAt:  now,
		Platform:   "youtube",
		RoomID:     platform.Str(videoID),
		LiveStatus: &status,
		Online:     online,
	}
}

// channelLiveItem channelId 锚点:id 稳定锚定频道,在播时 url 指向那场。
func channelLiveItem(ctx context.Context, sc *platform.ServiceContext, channelID string, now int64) (platform.Item, error) {
	live, err := fetchYoutubeChannelLive(ctx, sc, channelID)
	if err != nil {
		return platform.Item{}, err
	}
	var meta *liveMeta
	if live.VideoID != "" {
		meta = getYoutubeLiveMeta(ctx, sc, live.VideoID)
	}
	title := "YouTube 频道直播"
	if meta != nil && meta.Title != "" {
		title = meta.Title
	} else if live.ChannelName != "" {
		title = live.ChannelName
	}
	streamURL := "https://www.youtube.com/channel/" + channelID
	if live.VideoID != "" {
		streamURL = "https://www.youtube.com/watch?v=" + live.VideoID
	}
	roomID := live.VideoID
	if roomID == "" {
		roomID = channelID
	}
	var status platform.LiveStatus
	switch {
	case meta != nil && meta.IsLive:
		status = platform.LiveStatusLive
	case meta != nil:
		status = platform.LiveStatusOffline
	case live.VideoID != "":
		status = platform.LiveStatusLive
	default:
		status = platform.LiveStatusOffline
	}
	var author *platform.Author
	if live.ChannelName != "" {
		author = &platform.Author{Name: live.ChannelName}
	}
	var online *int
	if meta != nil {
		online = meta.Online
	}
	return platform.Item{
		ID:         "ylc-" + channelID,
		SourceID:   "youtube:live",
		Kind:       platform.KindLive,
		Title:      title,
		URL:        platform.Str(streamURL),
		Thumbnail:  platform.StrOmit(live.Thumb),
		Author:     author,
		FetchedAt:  now,
		Platform:   "youtube",
		RoomID:     platform.Str(roomID),
		LiveStatus: &status,
		Online:     online,
	}, nil
}

func newLiveHotChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:live-hot", Name: "YouTube 直播热门", Kind: platform.KindLive,
		DefaultInfo: platform.SourceInfo{},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			shelves, err := fetchYoutubeLiveShelves(ctx, sc)
			if err != nil {
				return platform.PageItems{}, err
			}
			var hot *youtubeLiveShelf
			for i := range shelves {
				if shelves[i].Slug == "hot" {
					hot = &shelves[i]
					break
				}
			}
			if hot == nil {
				return platform.PageItems{}, errors.New("youtube:live-hot 找不到 Live Now 栏目（直播页布局可能已变）")
			}
			now := sc.NowMs()
			return platform.PageItems{Items: liveHitsToItems("youtube:live-hot", hot.Hits, now)}, nil
		},
	})
}

func newLiveCategoryChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:live-category", Name: "YouTube 直播栏目", Kind: platform.KindLive,
		InfoTpl: []platform.SourceInfoField{
			{Key: "category", Label: "栏目（news / gaming / sports）", Required: platform.Bool(true)},
		},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			slug := normalizeYoutubeLiveCategory(info["category"])
			if slug == "" {
				return platform.PageItems{}, errors.New("youtube:live-category 需要栏目（news / gaming / sports，或完整栏目名）")
			}
			if slug == "hot" {
				return platform.PageItems{}, errors.New("youtube:live-category 不接 Live Now，请改用 youtube:live-hot")
			}
			shelves, err := fetchYoutubeLiveShelves(ctx, sc)
			if err != nil {
				return platform.PageItems{}, err
			}
			var shelf *youtubeLiveShelf
			var available []string
			for i := range shelves {
				if shelves[i].Slug == slug {
					shelf = &shelves[i]
				} else if shelves[i].Slug != "hot" && len(shelves[i].Hits) > 0 {
					available = append(available, shelves[i].Slug)
				}
			}
			if shelf == nil {
				joined := strings.Join(available, " / ")
				if joined == "" {
					joined = "（暂无）"
				}
				return platform.PageItems{}, fmt.Errorf("youtube:live-category 没有栏目 %s；可用：%s", slug, joined)
			}
			now := sc.NowMs()
			return platform.PageItems{Items: liveHitsToItems("youtube:live-category", shelf.Hits, now)}, nil
		},
	})
}

func newLiveCategoriesChannel(sc *platform.ServiceContext) platform.DataChannel {
	return singlePage(platform.ChannelSpec{
		Key: "youtube:live-categories", Name: "YouTube 直播栏目目录", Kind: platform.KindCategory,
		DefaultInfo: platform.SourceInfo{},
		FetchPage: func(ctx context.Context, info platform.SourceInfo, page int) (platform.PageItems, error) {
			shelves, err := fetchYoutubeLiveShelves(ctx, sc)
			if err != nil {
				return platform.PageItems{}, err
			}
			now := sc.NowMs()
			items := []platform.Item{}
			for _, shelf := range shelves {
				if shelf.Slug == "hot" || len(shelf.Hits) == 0 {
					continue
				}
				var thumbnail *string
				if shelf.Hits[0].Thumb != "" {
					thumbnail = platform.Str(shelf.Hits[0].Thumb)
				}
				items = append(items, platform.Item{
					ID:              "youtube-shelf:" + shelf.Slug,
					SourceID:        "youtube:live-categories",
					Kind:            platform.KindCategory,
					Title:           liveNowPrefixRe.ReplaceAllString(shelf.Title, ""),
					URL:             platform.Str("https://www.youtube.com/live"),
					Summary:         platform.Str(fmt.Sprintf("%d 个直播", len(shelf.Hits))),
					Thumbnail:       thumbnail,
					FetchedAt:       now,
					CategoryChannel: platform.Str("youtube:live-category"),
					CategoryInfo:    map[string]string{"category": shelf.Slug},
				})
			}
			return platform.PageItems{Items: items}, nil
		},
	})
}

func liveHitsToItems(sourceID string, hits []youtubeLiveHit, now int64) []platform.Item {
	sorted := byViewers(hits)
	items := make([]platform.Item, 0, len(sorted))
	for _, hit := range sorted {
		items = append(items, youtubeLiveHitToLive(sourceID, hit, now))
	}
	return items
}

// youtubeLiveHitToLive 直播页条目 → Live item(live-hot / live-category 共用)。
func youtubeLiveHitToLive(sourceID string, hit youtubeLiveHit, now int64) platform.Item {
	thumb := hit.Thumb
	if thumb == "" {
		thumb = "https://i.ytimg.com/vi/" + hit.VideoID + "/hqdefault.jpg"
	}
	var author *platform.Author
	if hit.ChannelTitle != "" {
		author = &platform.Author{Name: hit.ChannelTitle}
	}
	status := platform.LiveStatusLive
	return platform.Item{
		ID:         hit.VideoID,
		SourceID:   sourceID,
		Kind:       platform.KindLive,
		Title:      hit.Title,
		URL:        platform.Str("https://www.youtube.com/watch?v=" + hit.VideoID),
		Thumbnail:  platform.Str(thumb),
		Poster:     platform.Str(thumb),
		Author:     author,
		FetchedAt:  now,
		Platform:   "youtube",
		RoomID:     platform.Str(hit.VideoID),
		LiveStatus: &status,
		Online:     hit.Viewers,
	}
}

// byViewers 观看数降序(缺观看数排最后,稳定排序)。
func byViewers(hits []youtubeLiveHit) []youtubeLiveHit {
	out := append([]youtubeLiveHit(nil), hits...)
	sort.SliceStable(out, func(i, j int) bool {
		return viewerCount(out[i]) > viewerCount(out[j])
	})
	return out
}

func viewerCount(hit youtubeLiveHit) int {
	if hit.Viewers == nil {
		return 0
	}
	return *hit.Viewers
}
