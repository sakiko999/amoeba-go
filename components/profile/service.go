// Profile 服务组件:主播/UP 主个人主页的数据面与 tab 打开。搜索页经
// window.mygo.call("Profile.Open", {namespace, item}) 从结果条目进入主页;
// 主页页面挂载后经 Profile.Target 取目标(与 player-h5 同模式),视频/回看
// 列表经 Profile.Videos 按平台映射到 <ns>.fetch 渠道(映射见 identity.go)。
// manifest 来自本目录 component.json(经 go:embed),Activate 注入网关调用面。
// Service(页面绑定面)与 component(拓扑参与面)分开:打开 tab 依赖宿主 UI,
// Opener 由 tabs.Manager 实现并在宿主构造时注入。

package profile

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mygo-host/platform"
)

//go:embed component.json
var manifestRaw []byte

// ProfileTarget 是 profile 页面 tab 的主页目标(页面经 Profile.Target 取到)。
// Live* 是进入时的搜索现场快照,页面直接渲染直播间卡片。
type ProfileTarget struct {
	InstanceID string `json:"instanceId"`
	Platform   string `json:"platform"` // 网关 namespace(bili/douyu/huya/twitch/youtube)
	Name       string `json:"name"`
	Avatar     string `json:"avatar,omitempty"`
	RoomID     string `json:"roomId,omitempty"` // 直播间 ID;twitch 为频道 login
	UID        string `json:"uid,omitempty"`    // bili uid / youtube channelId
	URL        string `json:"url,omitempty"`    // 直播间外链(Player.Play 的 url)
	LiveTitle  string `json:"liveTitle,omitempty"`
	LiveStatus string `json:"liveStatus,omitempty"` // live / offline / unknown / 空
	LiveOnline int    `json:"liveOnline,omitempty"`
	LiveCover  string `json:"liveCover,omitempty"`
}

// Opener 由 tabs.Manager 实现:打开(或复用)个人主页 tab,以及取回 tab 目标。
type Opener interface {
	OpenProfile(title string, target ProfileTarget) error
	ProfileTarget(instanceID string) (ProfileTarget, bool)
}

type Service struct {
	opener Opener
	gw     platform.OpCaller
}

// NewService 返回页面绑定用的 Service 与参与拓扑激活的组件包装。
func NewService(opener Opener) (*Service, platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, nil, err
	}
	s := &Service{opener: opener}
	return s, &component{s: s, m: m}, nil
}

type component struct {
	s *Service
	m platform.ServiceManifest
}

func (c *component) Manifest() platform.ServiceManifest { return c.m }

func (c *component) Activate(r platform.Registrar) error {
	caller, ok := r.(platform.OpCaller)
	if !ok {
		return errors.New("profile: 网关未暴露调用面(Registrar 缺 OpCaller)")
	}
	c.s.gw = caller
	return nil
}

type OpenParams struct {
	Namespace string        `json:"namespace"`
	Item      platform.Item `json:"item"`
}

// Open 从搜索结果条目打开作者主页:身份按平台从条目解析,tab 复用换目标。
func (s *Service) Open(_ context.Context, p OpenParams) error {
	p.Namespace = strings.TrimSpace(p.Namespace)
	if p.Namespace == "" {
		return errors.New("open 需要 namespace 参数")
	}
	target, err := targetOf(p.Namespace, p.Item)
	if err != nil {
		return err
	}
	return s.opener.OpenProfile(target.Name, target)
}

// Target 是主页页面的挂载入口:取本 tab 的目标(没有则 nil)。
func (s *Service) Target(_ context.Context, instanceID string) (*ProfileTarget, error) {
	t, ok := s.opener.ProfileTarget(instanceID)
	if !ok {
		return nil, nil
	}
	return &t, nil
}

type VideosParams struct {
	InstanceID string `json:"instanceId"`
	Cursor     string `json:"cursor,omitempty"` // 空 = 首页
}

type VideosResult struct {
	Items   []platform.Item `json:"items"`
	HasMore *bool           `json:"hasMore,omitempty"`
	Cursor  *string         `json:"cursor,omitempty"`
	Total   *int            `json:"total,omitempty"`
}

// Videos 拉主页的视频/回看列表(平台渠道路由见 identity.go,游标原样透传)。
func (s *Service) Videos(ctx context.Context, p VideosParams) (VideosResult, error) {
	target, ok := s.opener.ProfileTarget(p.InstanceID)
	if !ok {
		return VideosResult{}, errors.New("该 tab 没有个人主页目标")
	}
	if s.gw == nil {
		return VideosResult{}, errors.New("profile 未激活(缺网关调用面)")
	}
	channel, info, err := videosSource(target)
	if err != nil {
		return VideosResult{}, err
	}
	payload := map[string]any{"channel": channel, "info": info}
	if cursor := strings.TrimSpace(p.Cursor); cursor != "" {
		payload["cursor"] = cursor
	}
	params, err := json.Marshal(payload)
	if err != nil {
		return VideosResult{}, err
	}
	raw, err := s.gw.Call(ctx, target.Platform+".fetch", params)
	if err != nil {
		return VideosResult{}, err
	}
	switch v := raw.(type) {
	case platform.FetchPage:
		return VideosResult{Items: nonNilItems(v.Items), HasMore: v.HasMore, Cursor: v.Cursor, Total: v.Total}, nil
	case platform.FetchResult:
		return VideosResult{Items: nonNilItems(v.Items), Total: v.Total}, nil
	default:
		return VideosResult{}, fmt.Errorf("fetch 返回类型异常(%T)", raw)
	}
}

func nonNilItems(items []platform.Item) []platform.Item {
	if items == nil {
		return []platform.Item{}
	}
	return items
}
