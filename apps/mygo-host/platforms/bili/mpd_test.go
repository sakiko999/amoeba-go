// MPD 装配黄金值：与 TS buildMpd 的输出逐字节一致（含无 duration 时的双空格）。

package bili

import "testing"

func TestBuildMpdGolden(t *testing.T) {
	full := buildMpd(
		[]mpdVideoRep{{
			ID: "64", BaseURL: "https://cdn.example.com/v?a=1&b=2", Width: 1920, Height: 1080,
			Codecs: "avc1.640028", Bandwidth: 1234567, InitRange: "0-915", IndexRange: "916-2503",
		}},
		&mpdAudioRep{
			ID: "30280", BaseURL: "https://cdn.example.com/a?x=1&y=2", Codecs: "mp4a.40.2",
			Bandwidth: 128000, InitRange: "0-836", IndexRange: "837-2400",
		},
		123.4567, 1.5,
	)
	wantFull := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static" mediaPresentationDuration="PT123.457S" minBufferTime="PT1.500S">
  <Period>
    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">
<Representation id="64" mimeType="video/mp4" codecs="avc1.640028" bandwidth="1234567" width="1920" height="1080">
  <BaseURL>https://cdn.example.com/v?a=1&amp;b=2</BaseURL>
  <SegmentBase indexRange="916-2503">
    <Initialization range="0-915" />
  </SegmentBase>
</Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" segmentAlignment="true" startWithSAP="1">
<Representation id="30280" mimeType="audio/mp4" codecs="mp4a.40.2" bandwidth="128000">
  <BaseURL>https://cdn.example.com/a?x=1&amp;y=2</BaseURL>
  <SegmentBase indexRange="837-2400">
    <Initialization range="0-836" />
  </SegmentBase>
</Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	if full != wantFull {
		t.Errorf("full MPD 不一致:\n got=%q\nwant=%q", full, wantFull)
	}

	noAudio := buildMpd([]mpdVideoRep{{
		ID: "32", BaseURL: "https://cdn.example.com/v", Width: 640, Height: 360,
		Codecs: "avc1.64001E", Bandwidth: 128000, InitRange: "0-1", IndexRange: "2-3",
	}}, nil, 0, 0)
	wantNoAudio := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011" type="static"  minBufferTime="PT1.500S">
  <Period>
    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">
<Representation id="32" mimeType="video/mp4" codecs="avc1.64001E" bandwidth="128000" width="640" height="360">
  <BaseURL>https://cdn.example.com/v</BaseURL>
  <SegmentBase indexRange="2-3">
    <Initialization range="0-1" />
  </SegmentBase>
</Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	if noAudio != wantNoAudio {
		t.Errorf("无音频 MPD 不一致:\n got=%q\nwant=%q", noAudio, wantNoAudio)
	}
}

func TestEscXML(t *testing.T) {
	if got := escXML(`<&>"'`); got != "&lt;&amp;&gt;&quot;&apos;" {
		t.Errorf("escXML = %q", got)
	}
}
