// DASH 装配:从 adaptiveFormats 挑 avc1 视频轨与最高码率 AAC 音轨,每档视频
// 生成一个只含该档 + 公共音轨的 MPD(dash.js 无档可选,天然锁目标档)。
// 排序语义对齐 TS 的 ramda 组合(sortBy 稳定升序 + reverse;取最后一个)。

package youtube

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"mygo-host/platform"
)

var codecsRe = regexp.MustCompile(`codecs="([^"]+)"`)

func codecsFromMime(mimeType string) (string, bool) {
	match := codecsRe.FindStringSubmatch(mimeType)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

func isAvcVideo(format playerFormat) bool {
	mime := format.mimeType()
	return strings.HasPrefix(strings.ToLower(mime), "video/mp4") &&
		strings.Contains(strings.ToLower(mime), "avc1") &&
		format.InitRange != nil && format.IndexRange != nil &&
		(format.baseURL() != "" || format.url() != "")
}

func isAacAudio(format playerFormat) bool {
	mime := format.mimeType()
	return strings.HasPrefix(strings.ToLower(mime), "audio/mp4") &&
		strings.Contains(strings.ToLower(mime), "mp4a") &&
		format.InitRange != nil && format.IndexRange != nil &&
		(format.baseURL() != "" || format.url() != "")
}

type resolvedVideo struct {
	format playerFormat
	url    string
}

// youtubeDashStreams 从 adaptiveFormats 装配 DASH 全档位流(每档一个 MPD)。
func youtubeDashStreams(ctx context.Context, sc *platform.ServiceContext, adaptive []playerFormat, duration float64, headers map[string]string, videoID string) ([]platform.Stream, error) {
	var videos []resolvedVideo
	for _, format := range adaptive {
		if !isAvcVideo(format) {
			continue
		}
		if streamURL, ok := resolveFormatURL(ctx, sc, format, videoID); ok {
			videos = append(videos, resolvedVideo{format: format, url: streamURL})
		}
	}
	if len(videos) == 0 {
		return nil, nil
	}
	audioRep := highestAudioRep(ctx, sc, adaptive, videoID)

	// 稳定升序再反转:同高度档位顺序也随之反转(对齐 ramda sortBy + reverse)。
	sort.SliceStable(videos, func(i, j int) bool { return videos[i].format.height() < videos[j].format.height() })
	for i, j := 0, len(videos)-1; i < j; i, j = i+1, j-1 {
		videos[i], videos[j] = videos[j], videos[i]
	}

	streams := make([]platform.Stream, 0, len(videos))
	for _, item := range videos {
		codecs, ok := codecsFromMime(item.format.mimeType())
		if !ok {
			codecs = "avc1.640033"
		}
		height := item.format.height()
		rep := mpdVideoRep{
			ID:         item.format.itagID(),
			BaseURL:    item.url,
			Width:      item.format.width(),
			Height:     height,
			Codecs:     codecs,
			Bandwidth:  item.format.bitrate(),
			InitRange:  item.format.InitRange.spec(),
			IndexRange: item.format.IndexRange.spec(),
		}
		quality := item.format.qualityLabel()
		if quality == "" {
			quality = fmt.Sprintf("%dp", height)
		}
		streams = append(streams, platform.Stream{
			URL:          "",
			Format:       platform.Str("dash"),
			Headers:      headers,
			Quality:      platform.Str(quality),
			Rate:         platform.Int(height),
			DashManifest: platform.Str(buildMpd([]mpdVideoRep{rep}, audioRep, duration)),
		})
	}
	return streams, nil
}

// highestAudioRep 最高码率 AAC 音轨(升序稳定排序取最后 = 同码率取后者)。
func highestAudioRep(ctx context.Context, sc *platform.ServiceContext, adaptive []playerFormat, videoID string) *mpdAudioRep {
	var audios []playerFormat
	for _, format := range adaptive {
		if isAacAudio(format) {
			audios = append(audios, format)
		}
	}
	if len(audios) == 0 {
		return nil
	}
	sort.SliceStable(audios, func(i, j int) bool { return audios[i].bitrate() < audios[j].bitrate() })
	audio := audios[len(audios)-1]
	streamURL, ok := resolveFormatURL(ctx, sc, audio, videoID)
	if !ok {
		return nil
	}
	codecs, found := codecsFromMime(audio.mimeType())
	if !found {
		codecs = "mp4a.40.2"
	}
	return &mpdAudioRep{
		ID:         audio.itagID(),
		BaseURL:    streamURL,
		Codecs:     codecs,
		Bandwidth:  audio.bitrate(),
		InitRange:  audio.InitRange.spec(),
		IndexRange: audio.IndexRange.spec(),
	}
}
