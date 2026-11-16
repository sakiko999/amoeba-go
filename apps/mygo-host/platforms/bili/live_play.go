// bilibili 直播流解析（复刻 dart bilibili_site.dart）：先 getRoomPlayInfo(qn=10000)
// 读档位表（g_qn_desc + accept_qn），再逐档重发要 avc 直链。服务端对访客降档且
// 多请求档位会收敛，故按「实际档位」去重；契约：最高清晰度排最前。

package bili

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"mygo-host/platform"
)

const livePlayAPI = "https://api.live.bilibili.com/xlive/web-room/v2/index/getRoomPlayInfo"

func (c *client) resolveLivePlay(ctx context.Context, roomID string, info platform.ResolveInfo) ([]platform.Stream, error) {
	cookie := nonEmptyResolveCookie(info)
	referer := liveBase + "/"
	base := map[string]string{
		"room_id":  roomID,
		"protocol": "0,1",
		"format":   "0,1,2",
		"codec":    "0,1",
		"platform": "web",
	}
	probe, err := c.liveProbe(ctx, mergeParams(base, map[string]string{"qn": "10000"}), referer, cookie)
	if err != nil {
		return nil, err
	}
	qualities := extractLiveQualities(probe)
	if len(qualities) == 0 {
		return resolveLiveProbe(probe, roomID)
	}

	var streams []platform.Stream
	seen := map[int]bool{}
	for idx, q := range qualities {
		s, applied, err := c.liveStreamAt(ctx, base, q, referer, cookie)
		if err != nil {
			if idx == 0 {
				return nil, err
			}
			continue
		}
		if s == nil || seen[applied] {
			continue
		}
		seen[applied] = true
		name := q.name
		if applied != q.qn {
			name = fmt.Sprintf("%s(请求%s)", qnDescName(probe, applied), q.name)
		}
		quality, rate := name, applied
		s.Quality, s.Rate = &quality, &rate
		streams = append(streams, *s)
	}
	if len(streams) == 0 {
		return resolveLiveProbe(probe, roomID)
	}
	sortStreamsDescByRate(streams)
	return streams, nil
}

// liveStreamAt 单个档位请求（format:0,2 + codec:0 只要 avc），返回首条流与实际档位。
func (c *client) liveStreamAt(ctx context.Context, base map[string]string, q liveQuality, referer string, cookie *string) (*platform.Stream, int, error) {
	params := mergeParams(base, map[string]string{"format": "0,2", "codec": "0", "qn": strconv.Itoa(q.qn)})
	signed, err := c.signLiveParams(ctx, params, cookie)
	if err != nil {
		return nil, 0, err
	}
	data, err := c.liveProbeSigned(ctx, signed, referer, cookie)
	if err != nil {
		return nil, 0, err
	}
	list := parseLiveStreams(data)
	if len(list) == 0 {
		return nil, 0, nil
	}
	applied := extractLiveAppliedQn(data)
	if applied == 0 {
		applied = q.qn
	}
	first := list[0]
	return &first, applied, nil
}

func (c *client) liveProbe(ctx context.Context, params map[string]string, referer string, cookie *string) (*livePlayData, error) {
	signed, err := c.signLiveParams(ctx, params, cookie)
	if err != nil {
		return nil, err
	}
	return c.liveProbeSigned(ctx, signed, referer, cookie)
}

func (c *client) liveProbeSigned(ctx context.Context, signed, referer string, cookie *string) (*livePlayData, error) {
	raw, _, err := c.getJSON(ctx, livePlayAPI+"?"+signed,
		reqOptions{Referer: referer, Buvid: true, Cookie: cookie})
	if err != nil {
		return nil, err
	}
	var resp livePlayResp
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		return &livePlayData{}, nil
	}
	return resp.Data, nil
}

// liveQuality 一档直播清晰度（accept_qn + g_qn_desc 中文名）。
type liveQuality struct {
	qn   int
	name string
}

func extractLiveQualities(data *livePlayData) []liveQuality {
	codec := firstCodec(data)
	if codec == nil {
		return nil
	}
	var out []liveQuality
	for _, raw := range codec.AcceptQn {
		qn := jnum(raw)
		if !finite(qn) || qn <= 0 {
			continue
		}
		out = append(out, liveQuality{qn: int(qn), name: qnDescName(data, int(qn))})
	}
	return out
}

// extractLiveAppliedQn 服务端对访客降档后的 current_qn（无该字段返回 0）。
func extractLiveAppliedQn(data *livePlayData) int {
	codec := firstCodec(data)
	if codec == nil {
		return 0
	}
	n := jnumPtr(codec.CurrentQn)
	if !finite(n) || n <= 0 {
		return 0
	}
	return int(n)
}

func qnDescName(data *livePlayData, qn int) string {
	playurl := livePlayurlOf(data)
	if playurl != nil {
		for _, g := range playurl.GQnDesc {
			if jintPtr(g.Qn) == qn && deref(g.Desc) != "" {
				return *g.Desc
			}
		}
	}
	return fmt.Sprintf("档位%d", qn)
}

func livePlayurlOf(data *livePlayData) *livePlayurl {
	if data == nil || data.PlayurlInfo == nil {
		return nil
	}
	return data.PlayurlInfo.Playurl
}

func firstCodec(data *livePlayData) *liveCodec {
	playurl := livePlayurlOf(data)
	if playurl == nil || len(playurl.Stream) == 0 {
		return nil
	}
	formats := playurl.Stream[0].Format
	if len(formats) == 0 || len(formats[0].Codec) == 0 {
		return nil
	}
	return &formats[0].Codec[0]
}

// liveURL 展开后的一条直链候选。
type liveURL struct {
	url    string
	format string
	host   string
}

// codecToURLs 只保留 avc 流（HEVC 浏览器播不了）；format 由 base_url 后缀推断。
func codecToURLs(c liveCodec) []liveURL {
	if strings.ToLower(deref(c.CodecName)) != "avc" {
		return nil
	}
	baseURL := deref(c.BaseURL)
	if baseURL == "" {
		return nil
	}
	format := "live"
	if strings.Contains(baseURL, ".m3u8") || strings.Contains(baseURL, ".fmp4") {
		format = "hls"
	} else if strings.Contains(baseURL, ".flv") {
		format = "flv"
	}
	var out []liveURL
	for _, ui := range c.URLInfo {
		host := deref(ui.Host)
		if host == "" {
			continue
		}
		out = append(out, liveURL{url: host + baseURL + deref(ui.Extra), format: format, host: host})
	}
	return out
}

// parseLiveStreams 展开 stream→format→codec→url_info 并按 flv / mcdn 稳定性排序。
func parseLiveStreams(data *livePlayData) []platform.Stream {
	headers := map[string]string{"referer": liveBase + "/", "user-agent": bilibiliUA}
	var urls []liveURL
	if playurl := livePlayurlOf(data); playurl != nil {
		for _, s := range playurl.Stream {
			for _, f := range s.Format {
				for _, codec := range f.Codec {
					urls = append(urls, codecToURLs(codec)...)
				}
			}
		}
	}
	sort.SliceStable(urls, func(i, j int) bool {
		flvI, flvJ := boolInt(urls[i].format == "flv"), boolInt(urls[j].format == "flv")
		if flvI != flvJ {
			return flvI < flvJ
		}
		return boolInt(isCDNHost(urls[i].host)) < boolInt(isCDNHost(urls[j].host))
	})
	streams := make([]platform.Stream, len(urls))
	for i, u := range urls {
		format := u.format
		streams[i] = platform.Stream{URL: u.url, Format: &format, Headers: headers}
	}
	return streams
}

func isCDNHost(host string) bool {
	return strings.Contains(host, "mcdn") || strings.Contains(host, "scdn")
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// resolveLiveProbe probe 兜底：仍无可播流 → 抛错（空数组会被 player 误报成功 0 条）。
func resolveLiveProbe(data *livePlayData, roomID string) ([]platform.Stream, error) {
	streams := parseLiveStreams(data)
	if len(streams) == 0 {
		return nil, fmt.Errorf("bili:live 未找到可播流(房间 %s 可能未开播)", roomID)
	}
	applied := extractLiveAppliedQn(data)
	if applied == 0 {
		return streams, nil
	}
	name := qnDescName(data, applied)
	for i := range streams {
		quality, rate := name, applied
		streams[i].Quality = &quality
		streams[i].Rate = &rate
	}
	return streams, nil
}

func mergeParams(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}
