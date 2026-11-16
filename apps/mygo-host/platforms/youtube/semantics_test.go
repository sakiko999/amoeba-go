// ramda 语义基线的 Go 等价物(移植 tests/ramda-semantics.test.ts 的非直观语义):
// maxBy 相等取先出现者、pathOr 缺失兜底、sortBy 稳定性与 last、sortBy+reverse 的
// 同值倒序、稳定降序排序。Go 侧不依赖 ramda,这些语义由手写循环/稳定排序承担。

package youtube

import (
	"context"
	"strings"
	"testing"

	"mygo-host/platform"
)

func progressiveFormat(itag, height int, streamURL string) playerFormat {
	return playerFormat{
		Itag:   fptr(float64(itag)),
		Height: fptr(float64(height)),
		URL:    platform.Str(streamURL),
	}
}

func dashVideoFormat(itag, height int, streamURL string) playerFormat {
	return playerFormat{
		Itag:       fptr(float64(itag)),
		Height:     fptr(float64(height)),
		Width:      fptr(1920),
		URL:        platform.Str(streamURL),
		MimeType:   platform.Str(`video/mp4; codecs="avc1.640028"`),
		Bitrate:    fptr(3000000),
		InitRange:  &rangePair{Start: platform.Str("0"), End: platform.Str("100")},
		IndexRange: &rangePair{Start: platform.Str("100"), End: platform.Str("200")},
	}
}

func dashAudioFormat(itag, bitrate int, streamURL string) playerFormat {
	return playerFormat{
		Itag:       fptr(float64(itag)),
		URL:        platform.Str(streamURL),
		MimeType:   platform.Str(`audio/mp4; codecs="mp4a.40.2"`),
		Bitrate:    fptr(float64(bitrate)),
		InitRange:  &rangePair{Start: platform.Str("0"), End: platform.Str("100")},
		IndexRange: &rangePair{Start: platform.Str("100"), End: platform.Str("200")},
	}
}

func TestMaxByTieKeepsFirstSeen(t *testing.T) {
	formats := []playerFormat{
		progressiveFormat(999, 2160, "https://cdn/unknown"), // 未知 itag 被过滤
		progressiveFormat(37, 1080, "https://cdn/first"),
		progressiveFormat(38, 1080, "https://cdn/second"),
		progressiveFormat(22, 720, "https://cdn/third"),
	}
	best := pickProgressiveVideo(formats)
	if best == nil || best.url() != "https://cdn/first" || best.itagID() != 37 {
		t.Errorf("best = %+v", best)
	}
	higher := pickProgressiveVideo([]playerFormat{
		progressiveFormat(22, 720, "https://cdn/low"),
		progressiveFormat(37, 1080, "https://cdn/high"),
	})
	if higher == nil || higher.height() != 1080 {
		t.Errorf("higher = %+v", higher)
	}
}

func TestPathOrDefaultMissing(t *testing.T) {
	value := mustJSON(t, `{"a":null,"b":{"c":"val"},"zero":0}`)
	if text, ok := value.path("a", "b").asText(); ok || text != "" {
		t.Errorf("路径中途 null 应为缺失,得 %q ok=%v", text, ok)
	}
	if text, ok := value.path("missing", "x").asText(); ok {
		t.Errorf("缺失路径应为空,得 %q", text)
	}
	if text, ok := value.path("b", "c").asText(); !ok || text != "val" {
		t.Errorf("正常路径 = %q ok=%v", text, ok)
	}
}

func TestSortByLastTiePicksLater(t *testing.T) {
	sc := testContext(t)
	adaptive := []playerFormat{
		dashVideoFormat(137, 1080, "https://cdn/v1080"),
		dashAudioFormat(139, 48000, "https://cdn/a48"),
		dashAudioFormat(140, 128000, "https://cdn/a128"),
		dashAudioFormat(141, 128000, "https://cdn/a128b"),
	}
	streams, err := youtubeDashStreams(context.Background(), sc, adaptive, 0,
		map[string]string{"referer": "https://www.youtube.com/"}, "vid")
	if err != nil || len(streams) != 1 {
		t.Fatalf("streams = %d err=%v", len(streams), err)
	}
	manifest := derefPtr(streams[0].DashManifest)
	if !strings.Contains(manifest, `<Representation id="141" mimeType="audio/mp4"`) {
		t.Errorf("同码率应取后出现者(141): %s", manifest)
	}
	if strings.Contains(manifest, `<Representation id="140" mimeType="audio/mp4"`) {
		t.Errorf("不应取 140: %s", manifest)
	}
}

func TestSortByReverseTieOrder(t *testing.T) {
	sc := testContext(t)
	adaptive := []playerFormat{
		dashVideoFormat(136, 720, "https://cdn/a"),
		dashVideoFormat(298, 720, "https://cdn/b"),
		dashVideoFormat(137, 1080, "https://cdn/c"),
	}
	streams, err := youtubeDashStreams(context.Background(), sc, adaptive, 0,
		map[string]string{"referer": "https://www.youtube.com/"}, "vid")
	if err != nil || len(streams) != 3 {
		t.Fatalf("streams = %d err=%v", len(streams), err)
	}
	if *streams[0].Rate != 1080 || *streams[1].Rate != 720 || *streams[2].Rate != 720 {
		t.Errorf("rates = %d/%d/%d", *streams[0].Rate, *streams[1].Rate, *streams[2].Rate)
	}
	// sortBy 稳定升序 + reverse:同高度档位顺序反转(298 先于 136)。
	if !strings.Contains(derefPtr(streams[1].DashManifest), `id="298"`) ||
		!strings.Contains(derefPtr(streams[2].DashManifest), `id="136"`) {
		t.Error("同高度档位未按 sortBy+reverse 语义反转")
	}
}

func TestStableDescendingByViewers(t *testing.T) {
	hits := []youtubeLiveHit{
		{VideoID: "none"},
		{VideoID: "first", Viewers: iptr(100)},
		{VideoID: "top", Viewers: iptr(200)},
		{VideoID: "second", Viewers: iptr(100)},
	}
	sorted := byViewers(hits)
	want := []string{"top", "first", "second", "none"}
	for i, id := range want {
		if sorted[i].VideoID != id {
			t.Errorf("sorted[%d] = %q, want %q", i, sorted[i].VideoID, id)
		}
	}
	if hits[0].VideoID != "none" {
		t.Error("byViewers 不应修改原切片")
	}
}
