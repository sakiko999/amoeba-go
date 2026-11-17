// Player 服务组件:manifest 来自本目录 component.json(经 go:embed),
// Activate 取网关调用面(platform.OpCaller,platform 断言后注入)——页面
// window.mygo.call("Player.Play", {namespace, url, title}) 经 <ns>.play
// 解析流,选中原生可播的一路后交给宿主打开播放 tab。打开 tab 的 Opener
// 由宿主构造时注入(与 jsrunner 同:依赖宿主 UI,不进拓扑)。
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

// Opener 由 tabs.Manager 实现:打开(或复用)播放 tab,以及取回 H5 tab 的目标。
type Opener interface {
	OpenPlayer(title, url, quality string) error
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

type PlayResult struct {
	// Engine 是实际打开的播放器:native(ui.Video/IMFMediaEngine)或 h5(player-h5 页面)。
	Engine  string `json:"engine"`
	Format  string `json:"format,omitempty"`
	Quality string `json:"quality,omitempty"`
}

func (s *Service) Play(ctx context.Context, p PlayParams) (PlayResult, error) {
	p.Namespace = strings.TrimSpace(p.Namespace)
	p.URL = strings.TrimSpace(p.URL)
	if p.Namespace == "" || p.URL == "" {
		return PlayResult{}, errors.New("play 需要 namespace 与 url 参数")
	}
	if s.gw == nil {
		return PlayResult{}, errors.New("player 未激活(缺网关调用面)")
	}
	if p.Title == "" {
		p.Title = p.URL
	}
	params, err := json.Marshal(struct {
		URL string `json:"url"`
	}{p.URL})
	if err != nil {
		return PlayResult{}, err
	}
	raw, err := s.gw.Call(ctx, p.Namespace+".play", params)
	if err != nil {
		return PlayResult{}, err
	}
	result, ok := raw.(platform.PlayOpResult)
	if !ok {
		return PlayResult{}, fmt.Errorf("play 返回类型异常(%T)", raw)
	}
	if len(result.Streams) == 0 {
		return PlayResult{}, errors.New("平台没有返回可播流")
	}
	// 原生播放器(IMFMediaEngine)只接无自定义请求头的 HLS/渐进式;其余
	// (DASH、FLV、防盗链流)交给 H5 播放器(hls.js/dash.js/flv.js)。
	if stream := pickNative(result.Streams); stream != nil {
		quality := ""
		if stream.Quality != nil {
			quality = *stream.Quality
		}
		if err := s.opener.OpenPlayer(p.Title, stream.URL, quality); err != nil {
			return PlayResult{}, err
		}
		return PlayResult{Engine: "native", Format: formatName(stream), Quality: quality}, nil
	}
	target := H5Target{
		Title:     p.Title,
		Kind:      p.Kind,
		Namespace: p.Namespace,
		URL:       p.URL,
		Streams:   result.Streams,
	}
	if err := s.opener.OpenPlayerH5(p.Title, target); err != nil {
		return PlayResult{}, err
	}
	return PlayResult{Engine: "h5"}, nil
}

// Target 是 player-h5 页面的挂载入口:取本 tab 当前的播放目标(没有则 nil)。
func (s *Service) Target(ctx context.Context, instanceID string) (*H5Target, error) {
	t, ok := s.opener.Target(instanceID)
	if !ok {
		return nil, nil
	}
	return &t, nil
}

// pickNative 挑一路原生可播的流:hls > 渐进式/未标注;需要自定义请求头
// (OpenURL 不能带 header)或 DASH 的流一律不算。nil = 该结果不属于原生播放器。
func pickNative(streams []platform.Stream) *platform.Stream {
	best, bestScore := (*platform.Stream)(nil), 0
	for i := range streams {
		if score := nativeScore(&streams[i]); score > bestScore {
			best, bestScore = &streams[i], score
		}
	}
	return best
}

func nativeScore(s *platform.Stream) int {
	if len(s.Headers) > 0 {
		return 0
	}
	format := ""
	if s.Format != nil {
		format = *s.Format
	}
	if format == string(platform.FormatDASH) || s.DashManifest != nil {
		return 0
	}
	if format == string(platform.FormatHLS) {
		return 2
	}
	return 1 // progressive 与未标注按直连处理
}

func formatName(s *platform.Stream) string {
	if s.Format != nil && *s.Format != "" {
		return *s.Format
	}
	return string(platform.FormatProgressve)
}
