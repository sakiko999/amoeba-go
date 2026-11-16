// InnerTube search:搜频道(params=EgIQAg==)与搜正在直播(params=EgJAAQ==)。
// 与 player 同一套 WEB client context;结果取 sectionListRenderer 两层 contents。

package youtube

import (
	"context"
	"strconv"

	"mygo-host/platform"
)

func youtubeSearch(ctx context.Context, sc *platform.ServiceContext, keyword, params string) (*jsonValue, error) {
	visitorData, err := getVisitorData(ctx, sc)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"context": map[string]any{"client": map[string]any{
			"clientName": "WEB", "clientVersion": webClientVersion,
			"hl": "en", "gl": "US", "visitorData": visitorData,
		}},
		"query":  keyword,
		"params": params,
	}
	raw, err := postJSON(ctx, sc, youtubeiV1+"/search?prettyPrint=false", body, map[string]string{
		"user-agent": desktopChromeUA,
	})
	if err != nil {
		return nil, err
	}
	return parseJSONValue(raw)
}

func searchSections(root *jsonValue) []*jsonValue {
	return root.path("contents", "twoColumnSearchResultsRenderer", "primaryContents", "sectionListRenderer", "contents").items()
}

// searchYouTubeChannels 搜频道(匿名约 20 条;channelId 是 youtube 渠道的订阅参数)。
func searchYouTubeChannels(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]youtubeChannelHit, error) {
	root, err := youtubeSearch(ctx, sc, keyword, "EgIQAg==")
	if err != nil {
		return nil, err
	}
	out := []youtubeChannelHit{}
	for _, section := range searchSections(root) {
		for _, item := range section.path("itemSectionRenderer", "contents").items() {
			renderer := item.get("channelRenderer")
			if renderer == nil {
				continue
			}
			channelID := renderer.get("channelId").asJSString()
			if channelID == "" {
				continue
			}
			title := channelID
			if text, ok := ytText(renderer.get("title")); ok {
				title = text
			}
			subsText, _ := ytText(renderer.get("subscriberCountText"))
			videoCountText, _ := ytText(renderer.get("videoCountText"))
			out = append(out, youtubeChannelHit{
				ChannelID:      channelID,
				Title:          title,
				Thumb:          ytThumb(renderer.get("thumbnail")),
				SubsText:       subsText,
				VideoCountText: videoCountText,
			})
		}
	}
	return out, nil
}

// searchYouTubeLive 搜正在直播的视频(videoId 是 youtube:live 的订阅参数)。
func searchYouTubeLive(ctx context.Context, sc *platform.ServiceContext, keyword string) ([]youtubeLiveHit, error) {
	root, err := youtubeSearch(ctx, sc, keyword, "EgJAAQ==")
	if err != nil {
		return nil, err
	}
	out := []youtubeLiveHit{}
	for _, section := range searchSections(root) {
		for _, item := range section.path("itemSectionRenderer", "contents").items() {
			renderer := item.get("videoRenderer")
			if renderer == nil {
				continue
			}
			videoID := renderer.get("videoId").asJSString()
			if videoID == "" {
				continue
			}
			title := videoID
			if text, ok := ytText(renderer.get("title")); ok {
				title = text
			}
			channelTitle, ok := ytText(renderer.get("ownerText"))
			if !ok {
				channelTitle, _ = ytText(renderer.get("longBylineText"))
			}
			viewersText, _ := ytText(renderer.get("viewCountText"))
			out = append(out, youtubeLiveHit{
				VideoID:      videoID,
				Title:        title,
				ChannelTitle: channelTitle,
				Thumb:        ytThumb(renderer.get("thumbnail")),
				Viewers:      searchViewers(viewersText),
			})
		}
	}
	return out, nil
}

// searchViewers 直播搜索的 "1,234 watching" → 1234("Waiting" 等非数字为 nil)。
func searchViewers(text string) *int {
	digits := digitsOnly(text)
	if digits == "" {
		return nil
	}
	value, err := strconv.Atoi(digits)
	if err != nil || value <= 0 {
		return nil
	}
	return &value
}
