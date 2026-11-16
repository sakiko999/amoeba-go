// 豆瓣映射测试:fixture 为真实 API 响应(与 TS provider.test.ts 同一黄金集),
// 另覆盖 metadata 协议 ops 的注册面与「无搜索」行为。

package douban

import (
	"context"
	"os"
	"strings"
	"testing"

	"mygo-host/platform"
)

func fixture(t *testing.T, name string) platform.RawJSON {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMapSubjectTV(t *testing.T) {
	m, err := mapSubject(fixture(t, "douban_tv_3016187.json"), "3016187")
	if err != nil {
		t.Fatal(err)
	}
	if m.MediaType != platform.MediaTV || m.Title != "权力的游戏 第一季" {
		t.Errorf("mediaType/title = %s / %q", m.MediaType, m.Title)
	}
	if trimDeref(m.OriginalTitle) != "Game of Thrones Season 1" {
		t.Errorf("originalTitle = %q", trimDeref(m.OriginalTitle))
	}
	if strings.Join(m.Genres, "/") != "剧情/奇幻/冒险" {
		t.Errorf("genres = %v", m.Genres)
	}
	if len(m.Ratings) != 1 || m.Ratings[0].Source != "douban" || m.Ratings[0].Value != 9.5 ||
		m.Ratings[0].Votes == nil || *m.Ratings[0].Votes != 505366 {
		t.Errorf("ratings = %+v", m.Ratings)
	}
	wantID := platform.UniqueId{Source: "douban", Value: "3016187"}
	if len(m.UniqueIds) != 1 || m.UniqueIds[0] != wantID {
		t.Errorf("uniqueIds = %+v", m.UniqueIds)
	}
	if len(m.Actors) < 5 {
		t.Fatalf("actors = %+v", m.Actors)
	}
	if m.Actors[0].Name != "蒂莫西·范·帕腾" ||
		trimDeref(m.Actors[0].Role) != "导演" || m.Actors[0].Order != 0 {
		t.Errorf("actors[0] = %+v", m.Actors[0])
	}
	if m.Actors[4].Name != "肖恩·宾" || m.Actors[4].Order != 4 {
		t.Errorf("actors[4] = %+v", m.Actors[4])
	}
	if m.EpisodeCount == nil || *m.EpisodeCount != 10 {
		t.Errorf("episodeCount = %v", m.EpisodeCount)
	}
	if m.SeasonCount == nil || *m.SeasonCount != 1 {
		t.Errorf("seasonCount = %v", m.SeasonCount)
	}
	if m.PosterURL == nil || !strings.Contains(*m.PosterURL, "doubanio.com") {
		t.Errorf("posterUrl = %v", m.PosterURL)
	}
}

func TestMapSubjectMovie(t *testing.T) {
	m, err := mapSubject(fixture(t, "douban_movie_1292052.json"), "1292052")
	if err != nil {
		t.Fatal(err)
	}
	if m.MediaType != platform.MediaMovie || m.Title != "肖申克的救赎" {
		t.Errorf("mediaType/title = %s / %q", m.MediaType, m.Title)
	}
	if trimDeref(m.OriginalTitle) != "The Shawshank Redemption" {
		t.Errorf("originalTitle = %q", trimDeref(m.OriginalTitle))
	}
	if m.Premiered != nil {
		t.Errorf("premiered = %q(want nil)", trimDeref(m.Premiered))
	}
	if m.EpisodeCount != nil {
		t.Errorf("episodeCount = %v(want nil)", m.EpisodeCount)
	}
}

func TestMapSubjectMissingTitle(t *testing.T) {
	if _, err := mapSubject(platform.RawJSON(`{"id":"1"}`), "1"); err == nil {
		t.Fatal("缺标题应报错")
	}
}

func TestOpsSurface(t *testing.T) {
	spec := New(&platform.ServiceContext{})
	if spec.Namespace != "douban" || spec.Label != "豆瓣" {
		t.Errorf("namespace/label = %q / %q", spec.Namespace, spec.Label)
	}
	if len(spec.Tags) != 1 || spec.Tags[0] != "media.metadata" {
		t.Errorf("tags = %v", spec.Tags)
	}
	if len(spec.Ops) != 3 {
		t.Errorf("ops = %d(want 3)", len(spec.Ops))
	}
	for _, name := range []string{"search", "fetchMetadata", "fetchEpisodes"} {
		if _, ok := spec.Ops[name]; !ok {
			t.Errorf("缺少 op %s", name)
		}
	}
	if _, err := spec.Ops["search"].Invoke(context.Background(), nil); err == nil {
		t.Error("search 应报「没有稳定的搜索 API」")
	}
	result, err := spec.Ops["fetchEpisodes"].Invoke(context.Background(), platform.RawJSON(`{"id":"1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if episodes, ok := result.([]platform.EpisodeMetadata); !ok || len(episodes) != 0 {
		t.Errorf("fetchEpisodes = %#v(want 空数组)", result)
	}
}
