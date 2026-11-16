// playurl 全档位解析：DASH 优先（音视频分离 → 每档一条带 MPD 的 Stream），
// DASH 不可用回退 durl 渐进式 mp4（逐档重发，最高 720P）。URL 带 deadline 签名，
// 必须播放时懒解析，不缓存。

package bili

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

// resolvePlayURL bvid/aid+cid → 全档位可播流（最高清晰度排最前）。
func (c *client) resolvePlayURL(ctx context.Context, id, cid string, cookie *string) ([]platform.Stream, error) {
	referer := "https://www.bilibili.com/video/" + id
	headers := map[string]string{"referer": referer, "user-agent": bilibiliUA}
	base := fmt.Sprintf("%s/x/player/playurl?%s&cid=%s&platform=pc", apiMain, playurlIDParam(id), cid)

	probe, err := c.playurlData(ctx, base+"&qn=116&fnval=16", referer, cookie)
	if err != nil {
		return nil, err
	}
	if streams := dashStreamsWithMpd(probe, headers); len(streams) > 0 {
		return streams, nil
	}

	durlProbe, err := c.playurlData(ctx, base+"&qn=80", referer, cookie)
	if err != nil {
		return nil, err
	}
	streams, err := c.durlStreams(ctx, base, referer, cookie, durlQualities(durlProbe), headers)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		// 无可解析档位时再取一次默认档（与 TS 的第三次 qn=80 请求一致）。
		data, err := c.playurlData(ctx, base+"&qn=80", referer, cookie)
		if err != nil {
			return nil, err
		}
		if s, ok := firstDurlStream(data, headers); ok {
			streams = append(streams, s)
		}
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("bilibili playurl: no playable stream for %s", id)
	}
	sortStreamsDescByRate(streams)
	return streams, nil
}

// durlStreams 逐档请求；首档失败整体报错，其余档失败只少一个可选档位。
func (c *client) durlStreams(ctx context.Context, base, referer string, cookie *string,
	qualities []durlQuality, headers map[string]string) ([]platform.Stream, error) {
	var streams []platform.Stream
	for idx, q := range qualities {
		data, err := c.playurlData(ctx, fmt.Sprintf("%s&qn=%d", base, q.qn), referer, cookie)
		if err != nil {
			if idx == 0 {
				return nil, err
			}
			c.log("bili: 视频档位 %s(qn=%d) 解析失败,跳过: %v", q.name, q.qn, err)
			continue
		}
		s, ok := firstDurlStream(data, headers)
		if !ok {
			continue
		}
		quality, rate := q.name, q.qn
		s.Quality, s.Rate = &quality, &rate
		streams = append(streams, s)
	}
	return streams, nil
}

// playurlData 一次 playurl 请求；data 缺失时返回 nil。
func (c *client) playurlData(ctx context.Context, url, referer string, cookie *string) (*playurlData, error) {
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: referer, Cookie: cookie})
	if err != nil {
		return nil, err
	}
	var resp playurlResp
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// durlQuality 一档 durl 清晰度（qn + 服务端中文名）。
type durlQuality struct {
	qn   int
	name string
}

func durlQualities(data *playurlData) []durlQuality {
	if data == nil {
		return nil
	}
	var out []durlQuality
	for i, raw := range data.AcceptQuality {
		qn := jnum(raw)
		if !finite(qn) || qn <= 0 {
			continue
		}
		name := "档位" + platform.JSString(raw)
		if i < len(data.AcceptDescription) && data.AcceptDescription[i] != nil {
			name = platform.JSString(data.AcceptDescription[i])
		}
		out = append(out, durlQuality{qn: int(qn), name: name})
	}
	return out
}

func firstDurlStream(data *playurlData, headers map[string]string) (platform.Stream, bool) {
	if data == nil {
		return platform.Stream{}, false
	}
	for _, d := range data.Durl {
		if d.URL != nil && *d.URL != "" {
			return platform.Stream{URL: *d.URL, Format: platform.Str("mp4"), Headers: headers}, true
		}
	}
	return platform.Stream{}, false
}

// dashStreamsWithMpd DASH 成功即返回（每档一条 Stream，附该档 MPD）。
func dashStreamsWithMpd(data *playurlData, headers map[string]string) []platform.Stream {
	if data == nil || data.Dash == nil {
		return nil
	}
	dash := data.Dash
	audio := pickAudio(dash.Audio)
	duration, minBuffer := 0.0, 1.5
	if dash.Duration != nil {
		duration = *dash.Duration
	}
	if dash.MinBufferTime != nil {
		minBuffer = *dash.MinBufferTime
	}
	var audioRep *platform.MpdAudioRep
	if audio != nil {
		rep := toAudioRep(*audio)
		audioRep = &rep
	}
	var streams []platform.Stream
	for _, v := range dash.Video {
		if v.BaseURL == nil || *v.BaseURL == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(deref(v.Codecs)), "avc") {
			continue
		}
		qn := jintPtr(v.ID)
		quality := dashName(data, qn)
		rate := qn
		manifest := platform.BuildMpd([]platform.MpdVideoRep{toVideoRep(v)}, audioRep, duration, minBuffer)
		streams = append(streams, platform.Stream{
			URL:          "",
			Format:       platform.Str("dash"),
			Headers:      headers,
			Quality:      &quality,
			Rate:         &rate,
			DashManifest: &manifest,
		})
	}
	sortStreamsDescByRate(streams)
	return streams
}

// pickAudio 最高码率音轨（稳定升序取 last 的等价实现）。
func pickAudio(audios []dashAudio) *dashAudio {
	if len(audios) == 0 {
		return nil
	}
	idx := 0
	for i := 1; i < len(audios); i++ {
		if jnumPtr(audios[i].Bandwidth) >= jnumPtr(audios[idx].Bandwidth) {
			idx = i
		}
	}
	return &audios[idx]
}

// dashName qn → 档位中文名（accept_quality/accept_description 同索引）。
func dashName(data *playurlData, qn int) string {
	for i, raw := range data.AcceptQuality {
		if jint(raw) != qn {
			continue
		}
		if i < len(data.AcceptDescription) && data.AcceptDescription[i] != nil {
			return platform.JSString(data.AcceptDescription[i])
		}
		return fmt.Sprintf("档位%d", qn)
	}
	return fmt.Sprintf("档位%d", qn)
}

// toVideoRep DASH 轨道 → MPD Representation(ID/Bandwidth 十进制字符串化)。
func toVideoRep(v dashVideo) platform.MpdVideoRep {
	return platform.MpdVideoRep{
		ID:         platform.JSString(derefFloat(v.ID)),
		BaseURL:    deref(v.BaseURL),
		Width:      jintPtr(v.Width),
		Height:     jintPtr(v.Height),
		Codecs:     derefOr(v.Codecs, "avc1.640033"),
		Bandwidth:  strconv.Itoa(jintPtr(v.Bandwidth)),
		InitRange:  segmentRange(v.SegmentBase, v.SegmentLow, true, "0-915"),
		IndexRange: segmentRange(v.SegmentBase, v.SegmentLow, false, "916-2503"),
	}
}

func toAudioRep(a dashAudio) platform.MpdAudioRep {
	return platform.MpdAudioRep{
		ID:         platform.JSString(derefFloat(a.ID)),
		BaseURL:    deref(a.BaseURL),
		Codecs:     derefOr(a.Codecs, "mp4a.40.2"),
		Bandwidth:  strconv.Itoa(jintPtr(a.Bandwidth)),
		InitRange:  segmentRange(a.SegmentBase, a.SegmentLow, true, "0-836"),
		IndexRange: segmentRange(a.SegmentBase, a.SegmentLow, false, "837-2400"),
	}
}

func segmentRange(upper, lower *segmentBase, init bool, fallback string) string {
	for _, sb := range []*segmentBase{upper, lower} {
		if sb == nil {
			continue
		}
		value := sb.IndexRange
		if init {
			value = sb.Initialization
		}
		if value != nil {
			return *value
		}
	}
	return fallback
}

func derefFloat(p *float64) any {
	if p == nil {
		return float64(0)
	}
	return *p
}

// sortStreamsDescByRate 契约：最高清晰度排最前（player 默认选流取第一个）。
func sortStreamsDescByRate(streams []platform.Stream) {
	sort.SliceStable(streams, func(i, j int) bool {
		return streamRate(streams[i]) > streamRate(streams[j])
	})
}

func streamRate(s platform.Stream) int {
	if s.Rate == nil {
		return 0
	}
	return *s.Rate
}
