// MPD 装配(对齐 platform-kit runtime/utils/mpd.ts):每档 MPD 只含该档 video
// Representation(+ 公共最高音轨),dash.js 无档可选,天然锁目标档。
// ID/Bandwidth 用 string:TS 模板 `${id}`/`${bandwidth}` 的十进制输出由调用方
// 格式化,装配层不关心上游数值类型。输出与 TS 逐字节一致(含无 duration 时
// type="static" 后的双空格)。

package platform

import (
	"fmt"
	"strings"
)

// MpdVideoRep 视频 Representation。
type MpdVideoRep struct {
	ID         string
	BaseURL    string
	Width      int
	Height     int
	Codecs     string
	Bandwidth  string
	InitRange  string
	IndexRange string
}

// MpdAudioRep 音频 Representation。
type MpdAudioRep struct {
	ID         string
	BaseURL    string
	Codecs     string
	Bandwidth  string
	InitRange  string
	IndexRange string
}

func videoRepresentation(v MpdVideoRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%s" mimeType="video/mp4" codecs="%s" bandwidth="%s" width="%d" height="%d">`,
			v.ID, escXML(v.Codecs), v.Bandwidth, v.Width, v.Height),
		fmt.Sprintf(`  <BaseURL>%s</BaseURL>`, escXML(v.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, v.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, v.InitRange),
		`  </SegmentBase>`,
		`</Representation>`,
	}, "\n")
}

func audioRepresentation(a MpdAudioRep) string {
	return strings.Join([]string{
		fmt.Sprintf(`<Representation id="%s" mimeType="audio/mp4" codecs="%s" bandwidth="%s">`,
			a.ID, escXML(a.Codecs), a.Bandwidth),
		fmt.Sprintf(`  <BaseURL>%s</BaseURL>`, escXML(a.BaseURL)),
		fmt.Sprintf(`  <SegmentBase indexRange="%s">`, a.IndexRange),
		fmt.Sprintf(`    <Initialization range="%s" />`, a.InitRange),
		`  </SegmentBase>`,
		`</Representation>`,
	}, "\n")
}

// BuildMpd 拼 DASH MPD XML;duration 0 省略 mediaPresentationDuration,
// minBufferTime 0 回落 1.5。
func BuildMpd(videos []MpdVideoRep, audio *MpdAudioRep, duration, minBufferTime float64) string {
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
	if minBufferTime == 0 {
		minBufferTime = 1.5
	}
	lines := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static" %s minBufferTime="PT%.3fS">`, durationAttr, minBufferTime),
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

// escXML XML 转义(BaseURL 含 & 等)。
func escXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return strings.ReplaceAll(s, "'", "&apos;")
}
