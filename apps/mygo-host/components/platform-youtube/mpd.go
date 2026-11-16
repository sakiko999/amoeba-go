// MPD 装配器(对齐 platform-kit runtime/utils/mpd):每档 MPD 只含该档 video
// Representation + 公共最高音轨,dash.js 无档可选,天然锁目标档无 ABR 降档。

package youtube

import (
	"fmt"
	"strconv"
	"strings"
)

type mpdVideoRep struct {
	ID         int
	BaseURL    string
	Width      int
	Height     int
	Codecs     string
	Bandwidth  float64
	InitRange  string
	IndexRange string
}

type mpdAudioRep struct {
	ID         int
	BaseURL    string
	Codecs     string
	Bandwidth  float64
	InitRange  string
	IndexRange string
}

func videoRepresentation(rep mpdVideoRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%d" mimeType="video/mp4" codecs="%s" bandwidth="%s" width="%d" height="%d">`,
			rep.ID, escXML(rep.Codecs), formatNumber(rep.Bandwidth), rep.Width, rep.Height),
		fmt.Sprintf("  <BaseURL>%s</BaseURL>", escXML(rep.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, rep.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, rep.InitRange),
		"  </SegmentBase>",
		"</Representation>",
	}, "\n")
}

func audioRepresentation(rep mpdAudioRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%d" mimeType="audio/mp4" codecs="%s" bandwidth="%s">`,
			rep.ID, escXML(rep.Codecs), formatNumber(rep.Bandwidth)),
		fmt.Sprintf("  <BaseURL>%s</BaseURL>", escXML(rep.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, rep.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, rep.InitRange),
		"  </SegmentBase>",
		"</Representation>",
	}, "\n")
}

// buildMpd 拼 DASH MPD XML;duration 为 0 时不写 mediaPresentationDuration。
func buildMpd(videos []mpdVideoRep, audio *mpdAudioRep, duration float64) string {
	videoReps := make([]string, 0, len(videos))
	for _, rep := range videos {
		videoReps = append(videoReps, videoRepresentation(rep))
	}
	audioRep := ""
	if audio != nil {
		audioRep = audioRepresentation(*audio)
	}
	durationAttr := ""
	if duration != 0 {
		durationAttr = fmt.Sprintf(` mediaPresentationDuration="PT%sS"`, formatFixed3(duration))
	}
	parts := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static"%s minBufferTime="PT%sS">`,
			durationAttr, formatFixed3(1.5)),
		"  <Period>",
		`    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">`,
		strings.Join(videoReps, "\n"),
		"    </AdaptationSet>",
	}
	if audioRep != "" {
		parts = append(parts,
			"    <AdaptationSet mimeType=\"audio/mp4\" segmentAlignment=\"true\" startWithSAP=\"1\">\n"+audioRep+"\n    </AdaptationSet>")
	}
	parts = append(parts, "  </Period>", "</MPD>")
	// 过滤空片段(TS .filter(Boolean))
	filtered := parts[:0]
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "\n")
}

func formatFixed3(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

// formatNumber TS 模板里的数值输出(无多余小数位)。
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func escXML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
