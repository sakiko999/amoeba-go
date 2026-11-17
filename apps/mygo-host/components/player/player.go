// Package player 原生播放器:video.OpenURL 直接打开平台解析出的流地址。
// Windows 走 Media Foundation,HLS 原生可播且不受 WebView2 的 CORS 限制
// (twitch VOD 等无 ACAO 头的流由此可放);画面与控件由 native UI 绘制。
// 播放器方法要求主线程调用:流的打开延迟到首次 View 构建(必然主线程)。

package player

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// Theme 是对宿主主题存储的最小依赖(实时读,跟随宿主主题切换)。
type Theme interface {
	Background() string
	Foreground() string
}

// Player 一个播放 tab 的状态;生命周期 = tab entry。切走 tab 不暂停
// (有声音的视频离开视图仍继续播放,如同浏览器标签页),关闭 tab 即停止。
type Player struct {
	url     string
	quality string
	theme   Theme
	vp      *ui.VideoPlayer
	opened  bool
	err     error
}

// New 只记录参数;流在首次 View 构建时打开(主线程约束 + 打开即播的语义)。
func New(url, quality string, theme Theme) *Player {
	return &Player{url: url, quality: quality, theme: theme}
}

// Close 停止播放与解码;幂等。
func (p *Player) Close() {
	if p.vp != nil {
		p.vp.Close()
		p.vp = nil
	}
}

// View 播放器视图:画面区(加载/错误/画面)+ 底部控件条。
func (p *Player) View(c *ui.Context) {
	p.openOnce()
	ui.Column(c).Fill().Background(ui.Hex(p.theme.Background())).Children(func() {
		p.videoView(c)
		p.controlsView(c)
	})
}

func (p *Player) openOnce() {
	if p.opened {
		return
	}
	p.opened = true
	vp, err := video.OpenURL(p.url)
	if err != nil {
		p.err = err
		return
	}
	p.vp = vp // 新播放器在 Video 显示它时立即开始播放
}

func (p *Player) videoView(c *ui.Context) {
	fg := ui.Hex(p.theme.Foreground())
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		switch {
		case p.err != nil || p.vp.Err() != nil:
			// OpenURL 失败,或播放中出错(流断/无法解码)
			reason := p.err
			if reason == nil {
				reason = p.vp.Err()
			}
			ui.Text(c, "播放失败").Bold().FontSize(15).TextColor(fg)
			ui.Text(c, reason.Error()).TextColor(fg)
		case !p.vp.Ready():
			// Ring 颜色必须显式跟随宿主主题(缺省跟系统,系统暗+宿主亮时不可见)
			loading.Ring(c, loading.Options{Color: fg}).Label("正在打开流")
		default:
			ui.Video(c, p.vp).Fit(ui.Contain).Grow(1)
		}
	})
}

// controlsView 底部控件:播放/暂停、进度(点播;直播显示「直播」)、静音、档位。
// 查询式控件(Button/Slider)在每次 View 构建中读状态,事件帧内执行动作。
func (p *Player) controlsView(c *ui.Context) {
	fg := ui.Hex(p.theme.Foreground())
	ui.Row(c).Height(52).AlignItems(ui.Center).Gap(12).Padding(0, 12).Children(func() {
		if p.vp == nil {
			return
		}
		pauseLabel := "暂停"
		if !p.vp.Playing() {
			pauseLabel = "播放"
		}
		if ui.Button(c, pauseLabel).Key("play").Clicked() {
			if p.vp.Playing() {
				p.vp.Pause()
			} else {
				p.vp.Play()
			}
		}
		if p.vp.Buffering() {
			ui.Text(c, "缓冲中…").TextColor(fg)
		}
		if dur := p.vp.Duration(); dur > 0 {
			p.seekView(c, dur, fg)
		} else {
			ui.Text(c, "直播").Bold().TextColor(fg) // 直播流 Duration 为 0
		}
		muteLabel := "静音"
		if p.vp.Muted() {
			muteLabel = "取消静音"
		}
		if ui.Button(c, muteLabel).Key("mute").Clicked() {
			p.vp.SetMuted(!p.vp.Muted())
		}
		if p.quality != "" {
			ui.Text(c, p.quality).TextColor(fg)
		}
	})
}

// seekView 点播进度:拖动 Slider seek;位置文本随播放推进(每帧重建读 Position)。
func (p *Player) seekView(c *ui.Context, dur time.Duration, fg ui.Color) {
	pos := p.vp.Position().Seconds()
	if ui.Slider(c, &pos, 0, dur.Seconds()).Key("seek").Label("进度").Grow(1).Changed() {
		p.vp.Seek(time.Duration(pos * float64(time.Second)))
	}
	ui.Text(c, fmtClock(p.vp.Position())+" / "+fmtClock(dur)).TextColor(fg)
}

func fmtClock(d time.Duration) string {
	d = d.Round(time.Second)
	if h := d / time.Hour; h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, d%time.Hour/time.Minute, d%time.Minute/time.Second)
	}
	return fmt.Sprintf("%d:%02d", d/time.Minute, d%time.Minute/time.Second)
}
