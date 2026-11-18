// 平台身份解析与数据源映射:搜索条目 → ProfileTarget(名字/头像/直播间/身份),
// 以及主页视频列表 → 各平台的 fetch 渠道。首版覆盖当前搜索支持的五个平台:
// bili(UP 主投稿)、douyu(直播回看)、twitch(频道回放)、youtube(频道视频),
// huya 暂无回看渠道(明确报错,页面显示空态说明)。

package profile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"mygo-host/platform"
)

var (
	twitchPathRe     = regexp.MustCompile(`^(?:www\.)?twitch\.tv/([A-Za-z0-9_]+)/?$`)
	youtubeChannelRe = regexp.MustCompile(`youtube\.com/channel/([A-Za-z0-9_-]+)`)
)

// targetOf 从搜索条目解析主页目标并按平台校验身份字段。
func targetOf(ns string, item platform.Item) (ProfileTarget, error) {
	t := ProfileTarget{
		Platform:   ns,
		Name:       item.Title,
		URL:        ptrString(item.URL),
		RoomID:     ptrString(item.RoomID),
		LiveTitle:  item.Title,
		LiveStatus: stringOfStatus(item.LiveStatus),
		LiveOnline: ptrInt(item.Online),
		LiveCover:  ptrString(item.Thumbnail),
	}
	if item.Author != nil {
		if item.Author.Name != "" {
			t.Name = item.Author.Name
		}
		t.Avatar = ptrString(item.Author.Avatar)
		t.UID = ptrString(item.Author.Handle)
	}
	switch ns {
	case "bili", "douyu", "huya":
		// bili 视频列表要 uid、房间卡片要 roomId,任一存在即可打开(缺的部分页面显示空态)。
		if t.RoomID == "" && t.UID == "" {
			return t, fmt.Errorf("%s 条目缺少房间或用户标识", ns)
		}
	case "twitch":
		// 频道搜索命中没有 roomId,从 twitch.tv/<login> 链接取频道名。
		if t.RoomID == "" {
			t.RoomID = twitchLogin(t.URL)
		}
		if t.RoomID == "" {
			return t, errors.New("twitch 条目缺少频道名")
		}
	case "youtube":
		if t.UID == "" {
			t.UID = youtubeChannelID(t.URL)
		}
		if t.UID == "" {
			return t, errors.New("youtube 条目缺少频道 ID(当前条目不是频道搜索结果)")
		}
	default:
		return t, fmt.Errorf("平台 %s 不支持个人主页", ns)
	}
	return t, nil
}

// videosSource 视频/回看列表的平台渠道路由(键与各平台渠道 InfoTpl 对齐)。
func videosSource(t ProfileTarget) (string, platform.SourceInfo, error) {
	switch t.Platform {
	case "bili":
		if t.UID == "" {
			return "", nil, errors.New("B站主页缺少 UP 主 uid,无法拉投稿列表")
		}
		return "bili:user_video", platform.SourceInfo{"uid": t.UID}, nil
	case "douyu":
		if t.RoomID == "" {
			return "", nil, errors.New("斗鱼主页缺少房间号,无法拉回看列表")
		}
		return "douyu:replay", platform.SourceInfo{"roomId": t.RoomID}, nil
	case "twitch":
		if t.RoomID == "" {
			return "", nil, errors.New("twitch 主页缺少频道名,无法拉回放列表")
		}
		return "twitch:vod", platform.SourceInfo{"login": t.RoomID}, nil
	case "youtube":
		if t.UID == "" {
			return "", nil, errors.New("youtube 主页缺少频道 ID,无法拉视频列表")
		}
		return "youtube", platform.SourceInfo{"channelId": t.UID}, nil
	case "huya":
		return "", nil, errors.New("虎牙暂不支持视频/回看列表")
	default:
		return "", nil, fmt.Errorf("平台 %s 没有主页数据源", t.Platform)
	}
}

// twitchLogin 从 https://www.twitch.tv/<login> 取频道名。
func twitchLogin(raw string) string {
	path := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	m := twitchPathRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func youtubeChannelID(raw string) string {
	m := youtubeChannelRe.FindStringSubmatch(raw)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func stringOfStatus(s *platform.LiveStatus) string {
	if s == nil {
		return ""
	}
	return string(*s)
}

func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptrInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
