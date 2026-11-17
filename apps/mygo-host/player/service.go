// Player 宿主服务:页面经 window.mygo.call("Player.Play", {namespace, url, title})
// 触发;经 api-gateway 调 <ns>.play 解析流,选中原生可播的一路后交给宿主
// 打开播放 tab。选流规则见 pickStream。

package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mygo-host/components/api-gateway"
	"mygo-host/platform"
)

// Opener 由 tabs.Manager 实现:打开(或复用)原生播放 tab。
type Opener interface {
	OpenPlayer(title, url, quality string) error
}

type Service struct {
	gw     *gateway.Gateway
	opener Opener
}

func NewService(gw *gateway.Gateway, opener Opener) *Service {
	return &Service{gw: gw, opener: opener}
}

type PlayParams struct {
	Namespace string `json:"namespace"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"` // tab 标题,缺省用 url
}

type PlayResult struct {
	Format  string `json:"format"`
	Quality string `json:"quality,omitempty"`
}

func (s *Service) Play(ctx context.Context, p PlayParams) (PlayResult, error) {
	p.Namespace = strings.TrimSpace(p.Namespace)
	p.URL = strings.TrimSpace(p.URL)
	if p.Namespace == "" || p.URL == "" {
		return PlayResult{}, errors.New("play 需要 namespace 与 url 参数")
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
	stream := pickStream(result.Streams)
	if stream == nil {
		return PlayResult{}, errors.New("没有原生可播的流(仅 DASH,或全部需要自定义请求头)")
	}
	quality := ""
	if stream.Quality != nil {
		quality = *stream.Quality
	}
	if err := s.opener.OpenPlayer(p.Title, stream.URL, quality); err != nil {
		return PlayResult{}, err
	}
	return PlayResult{Format: formatName(stream), Quality: quality}, nil
}

// pickStream 选原生可播流,打分高者胜:hls > 直连/未标注 > 需要自定义请求头
// 的流(OpenURL 不能带 header,播放大概率被拒,仅作无选之选);DASH 淘汰。
func pickStream(streams []platform.Stream) *platform.Stream {
	best, bestScore := (*platform.Stream)(nil), -1
	for i := range streams {
		if score := streamScore(&streams[i]); score > bestScore {
			best, bestScore = &streams[i], score
		}
	}
	return best
}

func streamScore(s *platform.Stream) int {
	format := ""
	if s.Format != nil {
		format = *s.Format
	}
	if format == string(platform.FormatDASH) || s.DashManifest != nil {
		return -1
	}
	score := 2
	if format != string(platform.FormatHLS) { // progressive 与未标注按直连处理
		score = 1
	}
	if len(s.Headers) > 0 {
		score -= 2
	}
	return score
}

func formatName(s *platform.Stream) string {
	if s.Format != nil && *s.Format != "" {
		return *s.Format
	}
	return string(platform.FormatProgressve)
}
