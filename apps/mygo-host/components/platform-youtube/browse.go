// YouTube 频道 InnerTube browse 解析:Videos tab(含频道会员标记)与 Live tab
// (频道当前在播定位)。两种响应布局兼容:新版 lockupViewModel 与旧版 videoRenderer;
// 递归按文档顺序收集(去重),continuation 令牌取首个。

package youtube

import (
	"context"
	"regexp"
	"strings"

	"mygo-host/platform"
)

const (
	videosTabParams = "EgZ2aWRlb3PyBgQKAjoA"
	liveTabParams   = "EgdzdHJlYW1z8gYECgJ6AA%3D%3D"
)

var (
	viewsTextRe   = regexp.MustCompile(`(?i)\bviews?\b`)
	agoTextRe     = regexp.MustCompile(`(?i)\bago\b`)
	membersOnlyRe = regexp.MustCompile(`(?i)members?\s*only`)
)

func fetchYoutubeChannelVideos(ctx context.Context, sc *platform.ServiceContext, channelID, continuation string) (youtubeChannelVideosPage, error) {
	opts := browseOptions{params: videosTabParams}
	if continuation != "" {
		opts = browseOptions{continuation: continuation}
	}
	raw, err := youtubeBrowse(ctx, sc, channelID, opts)
	if err != nil {
		return youtubeChannelVideosPage{}, err
	}
	return parseYoutubeChannelVideos(raw), nil
}

// parseYoutubeChannelVideos InnerTube 响应 → 视频列表 + 下一页令牌(纯函数)。
func parseYoutubeChannelVideos(raw *jsonValue) youtubeChannelVideosPage {
	videos := []youtubeChannelVideo{}
	seen := map[string]bool{}
	push := func(video *youtubeChannelVideo) {
		if video == nil || seen[video.VideoID] {
			return
		}
		seen[video.VideoID] = true
		videos = append(videos, *video)
	}
	walkJSON(raw, func(node *jsonValue) {
		if videoRenderer := node.get("videoRenderer"); videoRenderer != nil {
			push(fromVideoRenderer(videoRenderer))
		} else if lockup := node.get("lockupViewModel"); lockup != nil {
			push(fromLockup(lockup))
		}
	})
	if continuation := continuationOf(raw); continuation != "" {
		return youtubeChannelVideosPage{Videos: videos, Continuation: &continuation}
	}
	return youtubeChannelVideosPage{Videos: videos}
}

func fromLockup(lockup *jsonValue) *youtubeChannelVideo {
	videoID := lockup.get("contentId").asJSString()
	// 频道页同一批里还有 playlist / channel lockup,只收视频。
	if videoID == "" || !strings.Contains(lockup.get("contentType").asJSString(), "VIDEO") {
		return nil
	}
	metadata := lockup.path("metadata", "lockupMetadataViewModel")
	rows := metadata.path("metadata", "contentMetadataViewModel", "metadataRows").items()
	viewsText := ""
	publishedText := ""
	membersOnly := false
	for _, row := range rows {
		for _, part := range row.get("metadataParts").items() {
			text, ok := channelText(part.get("text"))
			if !ok || text == "" {
				continue
			}
			if viewsTextRe.MatchString(text) {
				if viewsText == "" {
					viewsText = text
				}
			} else if agoTextRe.MatchString(text) {
				if publishedText == "" {
					publishedText = text
				}
			}
		}
		for _, badge := range row.get("badges").items() {
			if badgeText, ok := badge.path("badgeViewModel", "badgeText").asText(); ok && membersOnlyRe.MatchString(badgeText) {
				membersOnly = true
			}
		}
	}
	title := videoID
	if text, ok := channelText(metadata.get("title")); ok {
		title = text
	}
	return &youtubeChannelVideo{
		VideoID:       videoID,
		Title:         title,
		Thumb:         channelThumb(lockup.path("contentImage", "thumbnailViewModel", "image")),
		ViewsText:     viewsText,
		PublishedText: publishedText,
		MembersOnly:   membersOnly,
	}
}

func fromVideoRenderer(videoRenderer *jsonValue) *youtubeChannelVideo {
	videoID := videoRenderer.get("videoId").asJSString()
	if videoID == "" {
		return nil
	}
	membersOnly := false
	for _, badge := range videoRenderer.get("badges").items() {
		if style, ok := badge.path("metadataBadgeRenderer", "style").asText(); ok && style == "BADGE_STYLE_TYPE_MEMBERS_ONLY" {
			membersOnly = true
		}
	}
	title := videoID
	if text, ok := channelText(videoRenderer.get("title")); ok {
		title = text
	}
	viewsText, _ := channelText(videoRenderer.get("viewCountText"))
	publishedText, _ := channelText(videoRenderer.get("publishedTimeText"))
	return &youtubeChannelVideo{
		VideoID:       videoID,
		Title:         title,
		Thumb:         channelThumb(videoRenderer.get("thumbnail")),
		ViewsText:     viewsText,
		PublishedText: publishedText,
		MembersOnly:   membersOnly,
	}
}

func continuationOf(root *jsonValue) string {
	token := ""
	walkJSON(root, func(node *jsonValue) {
		if token != "" {
			return
		}
		if value, ok := node.path("continuationItemRenderer", "continuationEndpoint", "continuationCommand", "token").asText(); ok && value != "" {
			token = value
		}
	})
	return token
}

func fetchYoutubeChannelLive(ctx context.Context, sc *platform.ServiceContext, channelID string) (youtubeChannelLive, error) {
	raw, err := youtubeBrowse(ctx, sc, channelID, browseOptions{params: liveTabParams})
	if err != nil {
		return youtubeChannelLive{}, err
	}
	return parseYoutubeChannelLive(raw), nil
}

// parseYoutubeChannelLive Live tab 响应 → 当前在播场(纯函数)。在播判据:
// 新版 lockup 的 badgeStyle 含 _LIVE 或文本 LIVE;旧版 videoRenderer iconType === "LIVE"。
func parseYoutubeChannelLive(raw *jsonValue) youtubeChannelLive {
	out := youtubeChannelLive{ChannelName: channelNameOf(raw)}
	walkJSON(raw, func(node *jsonValue) {
		if out.VideoID != "" {
			return
		}
		overlays := node.path("contentImage", "thumbnailViewModel", "overlays").items()
		for _, overlay := range overlays {
			for _, badge := range overlay.path("thumbnailBottomOverlayViewModel", "badges").items() {
				thumbnailBadge := badge.get("thumbnailBadgeViewModel")
				if thumbnailBadge == nil {
					continue
				}
				badgeStyle, _ := thumbnailBadge.get("badgeStyle").asText()
				badgeText, _ := thumbnailBadge.get("text").asText()
				isLive := strings.Contains(badgeStyle, "_LIVE") || strings.ToUpper(badgeText) == "LIVE"
				if !isLive {
					continue
				}
				if id, ok := node.get("contentId").asText(); ok && id != "" {
					out.VideoID = id
					out.Thumb = channelThumb(node.path("contentImage", "thumbnailViewModel", "image"))
				}
			}
		}
		videoRenderer := node.get("videoRenderer")
		if out.VideoID != "" || videoRenderer == nil {
			return
		}
		videoID, ok := videoRenderer.get("videoId").asText()
		if !ok || videoID == "" {
			return
		}
		for _, badge := range videoRenderer.get("badges").items() {
			if iconType, ok := badge.path("metadataBadgeRenderer", "iconType").asText(); ok && iconType == "LIVE" {
				out.VideoID = videoID
				out.Thumb = channelThumb(videoRenderer.get("thumbnail"))
				break
			}
		}
	})
	return out
}

// channelNameOf browse header → 频道名(新版 pageHeaderViewModel / 旧版 c4TabbedHeaderRenderer)。
func channelNameOf(raw *jsonValue) string {
	header := raw.get("header")
	name, ok := header.path("pageHeaderRenderer", "content", "pageHeaderViewModel", "title", "dynamicTextViewModel", "text", "content").asText()
	if !ok {
		name, ok = header.path("c4TabbedHeaderRenderer", "title", "simpleText").asText()
	}
	if !ok {
		name, ok = header.path("c4TabbedHeaderRenderer", "title").asText()
	}
	if ok && name != "" {
		return name
	}
	return ""
}
