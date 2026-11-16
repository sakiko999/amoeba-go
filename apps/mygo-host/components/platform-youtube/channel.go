// YoutubeChannel 渠道(官方 RSS / InnerTube 频道 Videos tab):单视频订阅与频道订阅。
// 主路径 InnerTube 带会员标记;失败回退官方 Atom feed(条目仍出,无 membersOnly)。
// continuation 翻页为上游 token 翻页(非数值游标),由自定义 FetchMore 承担。

package youtube

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	channelKey  = "youtube"
	channelName = "YouTube 频道"
)

type youtubeChannel struct {
	sc *platform.ServiceContext
}

func newYoutubeChannel(sc *platform.ServiceContext) platform.DataChannel {
	return &youtubeChannel{sc: sc}
}

func (c *youtubeChannel) Key() string  { return channelKey }
func (c *youtubeChannel) Name() string { return channelName }
func (c *youtubeChannel) Kind() platform.Kind {
	return platform.KindVideo
}

func (c *youtubeChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{
		{Key: "videoId", Label: "视频/直播 ID(可选)", Required: platform.Bool(false)},
		{Key: "channelId", Label: "频道 ID(可选,与 videoId 二选一)", Required: platform.Bool(false)},
	}
}

func (c *youtubeChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *youtubeChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &youtubeChannelSource{sc: c.sc, info: info}
}

type youtubeChannelSource struct {
	sc   *platform.ServiceContext
	info platform.SourceInfo
}

func (s *youtubeChannelSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	items, err := s.fetchItems(ctx)
	if err != nil {
		return platform.FetchResult{}, err
	}
	return platform.FetchResult{Items: items}, nil
}

func (s *youtubeChannelSource) fetchItems(ctx context.Context) ([]platform.Item, error) {
	now := s.sc.NowMs()
	if videoID := s.info["videoId"]; videoID != "" {
		meta := getYoutubeLiveMeta(ctx, s.sc, videoID)
		return []platform.Item{videoItem(videoID, now, meta)}, nil
	}
	channelID := s.info["channelId"]
	if channelID == "" {
		return nil, errChannelNeedsSource
	}
	page, err := fetchYoutubeChannelVideos(ctx, s.sc, channelID, "")
	if err == nil && len(page.Videos) > 0 {
		return channelVideosToItems(page.Videos, now), nil
	}
	xml, err := s.sc.HTTP.GetText(ctx,
		"https://www.youtube.com/feeds/videos.xml?channel_id="+encodeURIComponent(channelID),
		map[string]string{"user-agent": desktopChromeUA})
	if err != nil {
		return nil, err
	}
	feed, err := parseFeed(xml)
	if err != nil {
		return nil, err
	}
	items := make([]platform.Item, 0, len(feed.Channel.Items))
	for _, entry := range feed.Channel.Items {
		items = append(items, entryToVideo(entry, now, "youtube"))
	}
	return items, nil
}

var errChannelNeedsSource = errors.New("youtube: 需要 channelId 或 videoId")

// FetchMore 上游 continuation 翻页;cursor 为空时先拉首页取令牌(条目丢弃)。
func (s *youtubeChannelSource) FetchMore(ctx context.Context, cursor string, existing int) (platform.FetchPage, error) {
	channelID := s.info["channelId"]
	if s.info["videoId"] != "" || channelID == "" {
		return platform.FetchPage{Items: []platform.Item{}}, nil
	}
	token := cursor
	if token == "" {
		first, err := fetchYoutubeChannelVideos(ctx, s.sc, channelID, "")
		if err != nil {
			return platform.FetchPage{}, err
		}
		if first.Continuation == nil {
			return platform.FetchPage{Items: []platform.Item{}}, nil
		}
		token = *first.Continuation
	}
	page, err := fetchYoutubeChannelVideos(ctx, s.sc, channelID, token)
	if err != nil {
		return platform.FetchPage{}, err
	}
	result := platform.FetchPage{Items: channelVideosToItems(page.Videos, s.sc.NowMs())}
	if page.Continuation != nil {
		result.Cursor = page.Continuation
		result.HasMore = platform.Bool(true)
	}
	return result, nil
}

func channelVideosToItems(videos []youtubeChannelVideo, now int64) []platform.Item {
	items := make([]platform.Item, 0, len(videos))
	for _, video := range videos {
		items = append(items, channelVideoToItem(video, now))
	}
	return items
}

// channelVideoToItem InnerTube 频道条目 → Video(不设 publishedAt,交给 fetchedAt 兜底)。
func channelVideoToItem(video youtubeChannelVideo, now int64) platform.Item {
	thumb := video.Thumb
	if thumb == "" {
		thumb = "https://i.ytimg.com/vi/" + video.VideoID + "/hqdefault.jpg"
	}
	var membersOnly *bool
	if video.MembersOnly {
		membersOnly = platform.Bool(true)
	}
	return platform.Item{
		ID:          video.VideoID,
		SourceID:    "youtube",
		Kind:        platform.KindVideo,
		Title:       video.Title,
		URL:         platform.Str("https://www.youtube.com/watch?v=" + video.VideoID),
		Summary:     platform.StrOmit(joinNonEmpty([]string{video.ViewsText, video.PublishedText}, " · ")),
		Thumbnail:   platform.Str(thumb),
		Poster:      platform.Str(thumb),
		FetchedAt:   now,
		MembersOnly: membersOnly,
	}
}

// videoItem 单视频/直播订阅的 Item(真实标题/频道名;失败用 videoId 占位)。
func videoItem(videoID string, now int64, meta *liveMeta) platform.Item {
	title := "YouTube " + videoID
	var author *platform.Author
	if meta != nil {
		if meta.Title != "" {
			title = meta.Title
		}
		if meta.Author != "" {
			author = &platform.Author{Name: meta.Author}
		}
	}
	thumb := "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"
	return platform.Item{
		ID:        videoID,
		SourceID:  "youtube",
		Kind:      platform.KindVideo,
		Title:     title,
		URL:       platform.Str("https://www.youtube.com/watch?v=" + videoID),
		Thumbnail: platform.Str(thumb),
		Poster:    platform.Str(thumb),
		Author:    author,
		FetchedAt: now,
	}
}

var watchVideoIDRe = regexp.MustCompile(`[?&]v=([\w-]{6,})`)

func videoIDFromLink(link string) string {
	match := watchVideoIDRe.FindStringSubmatch(link)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

// entryToVideo YouTube Atom feed 条目 → Video(youtube 与 youtube:playlist 共用)。
func entryToVideo(entry parsedItem, now int64, sourceID string) platform.Item {
	raw := entry.Raw
	videoID := ""
	videoIDPresent := false
	if text := raw.child("videoId").textValue(); text != nil {
		videoID, videoIDPresent = *text, true
	} else if extracted := videoIDFromLink(entry.Link); extracted != "" {
		videoID, videoIDPresent = extracted, true
	}
	watchURL := entry.Link
	if videoID != "" {
		watchURL = "https://www.youtube.com/watch?v=" + videoID
	}
	id := videoID
	if !videoIDPresent {
		if entry.GUID != nil {
			id = *entry.GUID
		} else {
			id = entry.Link
		}
	}
	title := "(untitled)"
	if entry.Title != nil {
		title = *entry.Title
	}
	mediaGroup := raw.child("group")
	summary := mediaGroup.child("description").textValue()
	// TS asString:@_url 为空串时输出 ""(非 undefined),故属性存在即输出
	// (含空串),缺失才省略。
	thumbnail := (*string)(nil)
	if url, ok := mediaGroup.child("thumbnail").attr("url"); ok {
		thumbnail = platform.Str(url)
	}
	var duration *float64
	if value, ok := mediaGroup.child("content").attr("duration"); ok {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			duration = &parsed
		}
	}
	var author *platform.Author
	if entry.Author != "" {
		author = &platform.Author{Name: entry.Author}
	}
	var publishedAt *int64
	if entry.PubDate != nil && *entry.PubDate != "" {
		publishedAt = parseDateMillis(*entry.PubDate)
	}
	return platform.Item{
		ID:          id,
		SourceID:    sourceID,
		Kind:        platform.KindVideo,
		Title:       title,
		URL:         platform.Str(watchURL),
		Summary:     summary,
		Thumbnail:   thumbnail,
		Poster:      thumbnail,
		Author:      author,
		PublishedAt: publishedAt,
		FetchedAt:   now,
		Duration:    duration,
	}
}

// parseDateMillis JS new Date(str).getTime() 的常用格式覆盖(RFC1123 / RFC3339)。
func parseDateMillis(value string) *int64 {
	layouts := []string{
		time.RFC1123, time.RFC1123Z, time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 MST", "2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			millis := parsed.UnixMilli()
			return &millis
		}
	}
	return nil
}

func joinNonEmpty(parts []string, separator string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, separator)
}
