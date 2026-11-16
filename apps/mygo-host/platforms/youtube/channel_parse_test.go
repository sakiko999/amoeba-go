// 频道 page 解析测试(移植 src/example/youtube-channel.check.ts 的离线断言):
// 新版 lockup 会员标记/标题/封面/观看数/相对时间/continuation;旧版 videoRenderer;
// 非视频 lockup 跳过。纯 fixture,不联网。

package youtube

import "testing"

func lockupEntry(videoID, title string, members bool, withThumb bool) map[string]any {
	row := map[string]any{
		"metadataParts": []any{
			map[string]any{"text": map[string]any{"content": "2.5M views"}},
			map[string]any{"text": map[string]any{"content": "9 days ago"}},
		},
	}
	if members {
		row["badges"] = []any{map[string]any{"badgeViewModel": map[string]any{"badgeText": "Members only"}}}
	}
	lockup := map[string]any{
		"contentId":   videoID,
		"contentType": "LOCKUP_CONTENT_TYPE_VIDEO",
		"metadata": map[string]any{"lockupMetadataViewModel": map[string]any{
			"title": map[string]any{"content": title},
			"metadata": map[string]any{"contentMetadataViewModel": map[string]any{
				"metadataRows": []any{row},
			}},
		}},
	}
	if withThumb {
		lockup["contentImage"] = map[string]any{"thumbnailViewModel": map[string]any{
			"image": map[string]any{"sources": []any{
				map[string]any{"url": "https://x/s.jpg", "width": 100},
				map[string]any{"url": "https://x/l.jpg", "width": 800},
			}},
		}}
	}
	return map[string]any{"lockupViewModel": lockup}
}

func gridPage(items ...any) map[string]any {
	return map[string]any{"contents": map[string]any{"twoColumnBrowseResultsRenderer": map[string]any{
		"tabs": []any{map[string]any{"tabRenderer": map[string]any{
			"content": map[string]any{"richGridRenderer": map[string]any{"contents": items}},
		}}},
	}}}
}

func TestParseYoutubeChannelVideosLockup(t *testing.T) {
	page := gridPage(
		map[string]any{"richItemRenderer": map[string]any{"content": lockupEntry("v1", "标题 v1", true, true)}},
		map[string]any{"richItemRenderer": map[string]any{"content": lockupEntry("v2", "标题 v2", false, false)}},
		map[string]any{"richItemRenderer": map[string]any{"content": lockupEntry("v1", "重复", true, true)}},
		map[string]any{"continuationItemRenderer": map[string]any{
			"continuationEndpoint": map[string]any{"continuationCommand": map[string]any{"token": "tok-next"}},
		}},
	)
	parsed := parseYoutubeChannelVideos(mustMarshal(t, page))
	if len(parsed.Videos) != 2 {
		t.Fatalf("去重后视频数 = %d", len(parsed.Videos))
	}
	first := parsed.Videos[0]
	if first.VideoID != "v1" || first.Title != "标题 v1" {
		t.Errorf("first = %+v", first)
	}
	if !first.MembersOnly || parsed.Videos[1].MembersOnly {
		t.Errorf("membersOnly 判定错误: %+v", parsed.Videos)
	}
	if first.Thumb != "https://x/l.jpg" {
		t.Errorf("封面应取最大宽,得 %q", first.Thumb)
	}
	if first.ViewsText != "2.5M views" || first.PublishedText != "9 days ago" {
		t.Errorf("views/published = %q / %q", first.ViewsText, first.PublishedText)
	}
	if parsed.Continuation == nil || *parsed.Continuation != "tok-next" {
		t.Errorf("continuation = %v", parsed.Continuation)
	}
}

func TestParseYoutubeChannelVideosLegacy(t *testing.T) {
	page := map[string]any{"contents": map[string]any{"sectionListRenderer": map[string]any{
		"contents": []any{map[string]any{"itemSectionRenderer": map[string]any{
			"contents": []any{
				map[string]any{"videoRenderer": map[string]any{
					"videoId": "v3",
					"title":   map[string]any{"runs": []any{map[string]any{"text": "旧布局"}}},
					"badges": []any{map[string]any{"metadataBadgeRenderer": map[string]any{
						"style": "BADGE_STYLE_TYPE_MEMBERS_ONLY",
					}}},
					"thumbnail":         map[string]any{"thumbnails": []any{map[string]any{"url": "https://x/legacy.jpg", "width": 320}}},
					"viewCountText":     map[string]any{"simpleText": "1K views"},
					"publishedTimeText": map[string]any{"simpleText": "1 hour ago"},
				}},
				map[string]any{"videoRenderer": map[string]any{
					"videoId": "v4",
					"title":   map[string]any{"simpleText": "公开"},
					"badges": []any{map[string]any{"metadataBadgeRenderer": map[string]any{
						"style": "BADGE_STYLE_TYPE_VERIFIED",
					}}},
				}},
			},
		}}},
	}}}
	parsed := parseYoutubeChannelVideos(mustMarshal(t, page))
	if len(parsed.Videos) != 2 {
		t.Fatalf("视频数 = %d", len(parsed.Videos))
	}
	if !parsed.Videos[0].MembersOnly || parsed.Videos[1].MembersOnly {
		t.Errorf("membersOnly 判定错误: %+v", parsed.Videos)
	}
	if parsed.Videos[0].Thumb != "https://x/legacy.jpg" {
		t.Errorf("旧版封面 = %q", parsed.Videos[0].Thumb)
	}
	if parsed.Continuation != nil {
		t.Errorf("不应有 continuation,得 %q", *parsed.Continuation)
	}
}

func TestParseYoutubeChannelVideosSkipsNonVideoLockup(t *testing.T) {
	page := map[string]any{"contents": map[string]any{"lockupViewModel": map[string]any{
		"contentId":   "PL123",
		"contentType": "LOCKUP_CONTENT_TYPE_PLAYLIST",
		"metadata": map[string]any{"lockupMetadataViewModel": map[string]any{
			"title": map[string]any{"content": "播放列表"},
		}},
	}}}
	parsed := parseYoutubeChannelVideos(mustMarshal(t, page))
	if len(parsed.Videos) != 0 {
		t.Errorf("playlist lockup 不应计入视频: %+v", parsed.Videos)
	}
}
