// Player 服务组件:manifest 来自本目录 component.json(经 go:embed),
// Activate 取网关调用面(platform.OpCaller,platform 断言后注入)——页面
// window.mygo.call("Player.Play", {namespace, url, title}) 经 <ns>.play
// 解析流,统一交给 player-h5 页面 tab 播放(H5 引擎覆盖 HLS/DASH/FLV 与
// 防盗链流,原生播放器已退役)。打开 tab 的 Opener 由宿主构造时注入
// (与 jsrunner 同:依赖宿主 UI,不进拓扑)。
// Service(页面绑定面)与 component(拓扑参与面)分开:Activate/Manifest
// 的参数不满足 BindAs 的 JSON 可编码校验。

package player

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

// H5Target 是 player-h5 页面 tab 的播放目标(页面经 Player.Target 取到)。streams
// 原样携带平台解析结果 —— 页面侧按需选流(hls.js/dash.js/flv.js),不再回查网关。
type H5Target struct {
	InstanceID string            `json:"instanceId"`
	Title      string            `json:"title,omitempty"`
	Kind       string            `json:"kind,omitempty"` // live / video 等条目 kind
	Namespace  string            `json:"namespace"`
	URL        string            `json:"url"`
	Streams    []platform.Stream `json:"streams"`
}

// Opener 由 tabs.Manager 实现:打开(或复用) player-h5 tab,以及取回 H5 tab 的目标。
type Opener interface {
	OpenPlayerH5(title string, target H5Target) error
	Target(instanceID string) (H5Target, bool)
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
		return errors.New("player: 网关未暴露调用面(Registrar 缺 OpCaller)")
	}
	c.s.gw = caller
	return nil
}

type PlayParams struct {
	Namespace string `json:"namespace"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"` // tab 标题,缺省用 url
	Kind      string `json:"kind,omitempty"`  // live / video,透传给 H5 播放器
}

// Play 经网关解析平台流,任务连同全部候选流交给 H5 播放页(单播放 tab 复用)。
func (s *Service) Play(ctx context.Context, p PlayParams) error {
	p.Namespace = strings.TrimSpace(p.Namespace)
	p.URL = strings.TrimSpace(p.URL)
	if p.Namespace == "" || p.URL == "" {
		return errors.New("play 需要 namespace 与 url 参数")
	}
	if s.gw == nil {
		return errors.New("player 未激活(缺网关调用面)")
	}
	if p.Title == "" {
		p.Title = p.URL
	}
	params, err := json.Marshal(struct {
		URL string `json:"url"`
	}{p.URL})
	if err != nil {
		return err
	}
	raw, err := s.gw.Call(ctx, p.Namespace+".play", params)
	if err != nil {
		return err
	}
	result, ok := raw.(platform.PlayOpResult)
	if !ok {
		return fmt.Errorf("play 返回类型异常(%T)", raw)
	}
	if len(result.Streams) == 0 {
		return errors.New("平台没有返回可播流")
	}
	target := H5Target{
		Title:     p.Title,
		Kind:      p.Kind,
		Namespace: p.Namespace,
		URL:       p.URL,
		Streams:   result.Streams,
	}
	return s.opener.OpenPlayerH5(p.Title, target)
}

// Target 是 player-h5 页面的挂载入口:取本 tab 当前的播放目标(没有则 nil)。
func (s *Service) Target(ctx context.Context, instanceID string) (*H5Target, error) {
	t, ok := s.opener.Target(instanceID)
	if !ok {
		return nil, nil
	}
	return &t, nil
}
