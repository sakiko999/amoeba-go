// DASH MPD 装配（对齐 platform-kit runtime/utils/mpd.ts）：每档 MPD 只含该档
// video Representation（+ 公共最高音轨），dash.js 无档可选，天然锁目标档。
// 输出字符串与 TS buildMpd 逐字节一致（含无 duration 时的双空格）。

package bili

import (
	"fmt"
	"strings"
)

type mpdVideoRep struct {
	ID         string
	BaseURL    string
	Width      int
	Height     int
	Codecs     string
	Bandwidth  int
	InitRange  string
	IndexRange string
}

type mpdAudioRep struct {
	ID         string
	BaseURL    string
	Codecs     string
	Bandwidth  int
	InitRange  string
	IndexRange string
}

func videoRepresentation(v mpdVideoRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%s" mimeType="video/mp4" codecs="%s" bandwidth="%d" width="%d" height="%d">`,
			v.ID, escXML(v.Codecs), v.Bandwidth, v.Width, v.Height),
		fmt.Sprintf(`  <BaseURL>%s</BaseURL>`, escXML(v.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, v.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, v.InitRange),
		`  </SegmentBase>`,
		`</Representation>`,
	}, "\n")
}

func audioRepresentation(a mpdAudioRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%s" mimeType="audio/mp4" codecs="%s" bandwidth="%d">`,
			a.ID, escXML(a.Codecs), a.Bandwidth),
		fmt.Sprintf(`  <BaseURL>%s</BaseURL>`, escXML(a.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, a.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, a.InitRange),
		`  </SegmentBase>`,
		`</Representation>`,
	}, "\n")
}

// buildMpd 拼 DASH MPD XML（duration 为 0 时省略 mediaPresentationDuration）。
func buildMpd(videos []mpdVideoRep, audio *mpdAudioRep, duration, minBufferTime float64) string {
	reps := make([]string, len(videos))
	for i, v := range videos {
		reps[i] = videoRepresentation(v)
	}
	videoReps := strings.Join(reps, "\n")
	audioRep := ""
	if audio != nil {
		audioRep = audioRepresentation(*audio)
	}
	durationAttr := ""
	if duration != 0 {
		durationAttr = fmt.Sprintf(`mediaPresentationDuration="PT%.3fS"`, duration)
	}
	buffer := minBufferTime
	if buffer == 0 {
		buffer = 1.5
	}
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static" %s minBufferTime="PT%.3fS">`, durationAttr, buffer),
		`  <Period>`,
		`    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">`,
		videoReps,
		`    </AdaptationSet>`,
	}
	if audioRep != "" {
		lines = append(lines,
			`    <AdaptationSet mimeType="audio/mp4" segmentAlignment="true" startWithSAP="1">`+"\n"+audioRep+"\n"+`    </AdaptationSet>`)
	}
	lines = append(lines, `  </Period>`, `</MPD>`)
	return strings.Join(lines, "\n")
}

func escXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return strings.ReplaceAll(s, "'", "&apos;")
}
