// tmdb 映射测试:fixture 来自真实 API 响应(与 TS provider.test.ts 同一黄金集)。

package tmdb

import (
	"encoding/json"
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

func TestMapTvDetail(t *testing.T) {
	m, err := mapTvDetail(fixture(t, "tmdb_tv_1399.json"), "1399")
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "权力的游戏" || deref(m.OriginalTitle) != "Game of Thrones" {
		t.Errorf("title = %q / %q", m.Title, deref(m.OriginalTitle))
	}
	if deref(m.Premiered) != "2011-04-17" || deref(m.Status) != "Ended" {
		t.Errorf("premiered/status = %q / %q", deref(m.Premiered), deref(m.Status))
	}
	if len(m.Genres) != 3 {
		t.Errorf("genres = %d", len(m.Genres))
	}
	found := false
	for _, s := range m.Studios {
		if s == "HBO" {
			found = true
		}
	}
	if !found {
		t.Errorf("studios missing HBO: %v", m.Studios)
	}
	if len(m.Ratings) != 1 || m.Ratings[0].Source != "themoviedb" || m.Ratings[0].Value != 8.471 || *m.Ratings[0].Votes != 27708 {
		t.Errorf("ratings = %+v", m.Ratings)
	}
	wantIDs := []platform.UniqueId{{Source: "tmdb", Value: "1399"}, {Source: "imdb", Value: "tt0944947"}}
	if len(m.UniqueIds) != 2 || m.UniqueIds[0] != wantIDs[0] || m.UniqueIds[1] != wantIDs[1] {
		t.Errorf("uniqueIds = %+v", m.UniqueIds)
	}
	if len(m.Actors) == 0 || m.Actors[0].Name != "彼特·丁拉基" || deref(m.Actors[0].Role) != "Tyrion 'The Halfman' Lannister" || m.Actors[0].Order != 0 {
		t.Errorf("actors[0] = %+v", m.Actors[0])
	}
	if *m.SeasonCount != 8 || *m.EpisodeCount != 73 {
		t.Errorf("season/episode count = %v / %v", m.SeasonCount, m.EpisodeCount)
	}
	if deref(m.PosterURL) == "" {
		t.Error("posterUrl empty")
	}
}

func TestMapMovieDetail(t *testing.T) {
	m, err := mapMovieDetail(fixture(t, "tmdb_movie_550.json"), "550")
	if err != nil {
		t.Fatal(err)
	}
	if m.MediaType != platform.MediaMovie || m.Title != "搏击俱乐部" || deref(m.OriginalTitle) != "Fight Club" {
		t.Errorf("title = %q / %q (%s)", m.Title, deref(m.OriginalTitle), m.MediaType)
	}
	if deref(m.Premiered) != "1999-10-15" {
		t.Errorf("premiered = %q", deref(m.Premiered))
	}
	if len(m.Ratings) == 0 || m.Ratings[0].Value != 8.437 {
		t.Errorf("ratings = %+v", m.Ratings)
	}
	found := false
	for _, id := range m.UniqueIds {
		if id.Source == "imdb" && id.Value == "tt0137523" {
			found = true
		}
	}
	if !found {
		t.Errorf("uniqueIds missing imdb: %+v", m.UniqueIds)
	}
}

func TestMapSearch(t *testing.T) {
	tv, err := mapSearch(fixture(t, "tmdb_search_tv.json"), platform.MediaTV, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(tv) != 1 {
		t.Fatalf("tv results = %d", len(tv))
	}
	if tv[0].ID != "229858" || tv[0].Title != "命运／奇异赝品" || deref(tv[0].Year) != "2024" || tv[0].MediaType != platform.MediaTV {
		t.Errorf("tv[0] = %+v", tv[0])
	}
	limited, _ := mapSearch(fixture(t, "tmdb_search_tv.json"), platform.MediaTV, 0)
	if len(limited) != 0 {
		t.Errorf("limit 0 results = %d", len(limited))
	}
	empty, _ := mapSearch(platform.RawJSON(`{"results":[]}`), platform.MediaMovie, 20)
	if len(empty) != 0 {
		t.Errorf("empty results = %d", len(empty))
	}
}

func TestMapSeasonEpisodes(t *testing.T) {
	eps, err := mapSeasonEpisodes(fixture(t, "tmdb_season_1399_s1.json"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 10 {
		t.Fatalf("episodes = %d", len(eps))
	}
	first := eps[0]
	if first.Season != 1 || first.Episode != 1 || first.Title != "凛冬将至" || deref(first.Aired) != "2011-04-17" || *first.RuntimeMinutes != 62 {
		t.Errorf("eps[0] = %+v", first)
	}
	if deref(first.ThumbURL) == "" {
		t.Error("thumbUrl empty")
	}
}

// 保证契约结构 JSON 形状稳定(字段名 camelCase、可选字段省略)。
func TestJSONShape(t *testing.T) {
	m, err := mapMovieDetail(fixture(t, "tmdb_movie_550.json"), "550")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"mediaType"`, `"title"`, `"genres"`, `"ratings"`, `"actors"`, `"uniqueIds"`, `"premiered"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("json missing %s: %s", key, raw)
		}
	}
}
