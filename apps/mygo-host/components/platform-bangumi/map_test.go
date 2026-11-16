// bangumi 映射测试:fixture 来自真实 API 响应(与 TS provider.test.ts 同一黄金集)。

package bangumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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

func TestMapSubject(t *testing.T) {
	m, err := mapSubject(fixture(t, "bangumi_subject_8.json"), "8", platform.MediaTV)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Code Geass 反叛的鲁路修R2" {
		t.Errorf("title = %q", m.Title)
	}
	if deref(m.OriginalTitle) != "コードギアス 反逆のルルーシュR2" {
		t.Errorf("originalTitle = %q", deref(m.OriginalTitle))
	}
	if deref(m.Premiered) != "2008-04-06" {
		t.Errorf("premiered = %q", deref(m.Premiered))
	}
	if len(m.Studios) != 1 || m.Studios[0] != "サンライズ" {
		t.Errorf("studios = %v", m.Studios)
	}
	if len(m.Ratings) != 1 || m.Ratings[0].Source != "bangumi" || m.Ratings[0].Value != 8.3 ||
		m.Ratings[0].Votes == nil || *m.Ratings[0].Votes != 18749 {
		t.Errorf("ratings = %+v", m.Ratings)
	}
	wantID := platform.UniqueId{Source: "bangumi", Value: "8"}
	if len(m.UniqueIds) != 1 || m.UniqueIds[0] != wantID {
		t.Errorf("uniqueIds = %+v", m.UniqueIds)
	}
	if m.EpisodeCount == nil || *m.EpisodeCount != 25 {
		t.Errorf("episodeCount = %v", m.EpisodeCount)
	}
	if m.SeasonCount == nil || *m.SeasonCount != 1 {
		t.Errorf("seasonCount = %v", m.SeasonCount)
	}
	if !strings.Contains(deref(m.PosterURL), "lain.bgm.tv") {
		t.Errorf("posterUrl = %q", deref(m.PosterURL))
	}
	if len(m.Genres) != 0 || len(m.Actors) != 0 || m.Status != nil {
		t.Errorf("unexpected extra fields: genres=%v actors=%v status=%v", m.Genres, m.Actors, m.Status)
	}
}

func TestMapSubjectRejectsBookAndMusic(t *testing.T) {
	cases := map[int]string{subjectTypeBook: "书籍", subjectTypeMusic: "音乐"}
	for subjectType, kind := range cases {
		raw := platform.RawJSON(fmt.Sprintf(`{"type":%d,"name":"x"}`, subjectType))
		_, err := mapSubject(raw, "1", platform.MediaTV)
		var providerErr *platform.ProviderError
		if !errors.As(err, &providerErr) {
			t.Fatalf("type %d: err = %v", subjectType, err)
		}
		if !strings.Contains(providerErr.Message, kind) {
			t.Errorf("type %d: message = %q", subjectType, providerErr.Message)
		}
	}
	if _, err := mapSubject(platform.RawJSON(`{"type":5,"name":"x"}`), "5", platform.MediaTV); err == nil {
		t.Error("unsupported type 5: expected error")
	}
}

func TestMapSubjectAliasFallback(t *testing.T) {
	raw := platform.RawJSON(`{"type":2,"name":"同名","name_cn":"同名","infobox":[{"key":"别名","value":[{"v":"别名A"}]}]}`)
	m, err := mapSubject(raw, "1", platform.MediaTV)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "别名A" {
		t.Errorf("title = %q", m.Title)
	}
	if deref(m.OriginalTitle) != "同名" {
		t.Errorf("originalTitle = %q", deref(m.OriginalTitle))
	}
}

func TestMapSearch(t *testing.T) {
	raw := fixture(t, "bangumi_search.json")
	all, err := mapSearch(raw, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("results = %d", len(all))
	}
	first := all[0]
	if first.ID != "402128" || first.Title != "命运-奇异赝品 黎明低语" ||
		deref(first.Year) != "2023" || first.MediaType != platform.MediaTV {
		t.Errorf("first = %+v", first)
	}
	if all[1].Title != "命运-奇异赝品" || deref(all[1].OriginalTitle) != "Fate/strange Fake" {
		t.Errorf("alias item = %+v", all[1])
	}
	if all[3].Year != nil {
		t.Errorf("empty date year = %q", deref(all[3].Year))
	}
	limited, err := mapSearch(raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Errorf("limited results = %d", len(limited))
	}
}

func TestMapEpisodes(t *testing.T) {
	eps, err := mapEpisodes(fixture(t, "bangumi_episodes_8.json"), "8")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 25 {
		t.Fatalf("episodes = %d", len(eps))
	}
	first := eps[0]
	if first.Season != 1 || first.Episode != 1 || first.Title != "魔王的苏醒之日" ||
		deref(first.OriginalTitle) != "魔神 が 目覚める 日" || deref(first.Aired) != "2008-04-06" ||
		first.RuntimeMinutes == nil || *first.RuntimeMinutes != 24 {
		t.Errorf("eps[0] = %+v", first)
	}
	foundEpisodeID := false
	for _, id := range first.UniqueIds {
		if id == (platform.UniqueId{Source: "bangumi-episode", Value: "522"}) {
			foundEpisodeID = true
		}
	}
	if !foundEpisodeID {
		t.Errorf("uniqueIds missing episode id: %+v", first.UniqueIds)
	}
	for i := 1; i < len(eps); i++ {
		if eps[i].Episode <= eps[i-1].Episode {
			t.Fatalf("episodes not ascending at %d: %d <= %d", i, eps[i].Episode, eps[i-1].Episode)
		}
	}
}

func TestMapEpisodesSkipsInvalidNumbers(t *testing.T) {
	raw := platform.RawJSON(`{"data":[
		{"ep":0,"name":"op"},
		{"ep":1.5,"name":"sp"},
		{"ep":2,"sort":2,"name_cn":"第二集"},
		{"ep":1,"name":"第一集"}
	]}`)
	eps, err := mapEpisodes(raw, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 2 || eps[0].Episode != 1 || eps[1].Episode != 2 {
		t.Fatalf("episodes = %+v", eps)
	}
	if eps[0].Title != "第一集" || eps[1].Title != "第二集" {
		t.Errorf("titles = %q / %q", eps[0].Title, eps[1].Title)
	}
	if len(eps[0].UniqueIds) != 1 || eps[0].UniqueIds[0].Value != "1" {
		t.Errorf("uniqueIds = %+v", eps[0].UniqueIds)
	}
}

// 保证契约结构 JSON 形状稳定(字段名 camelCase、集合字段为空数组、可选字段省略)。
func TestJSONShape(t *testing.T) {
	m, err := mapSubject(fixture(t, "bangumi_subject_8.json"), "8", platform.MediaTV)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, key := range []string{`"mediaType"`, `"title"`, `"genres":[]`, `"studios"`, `"ratings"`, `"actors":[]`, `"uniqueIds"`, `"premiered"`, `"posterUrl"`} {
		if !strings.Contains(text, key) {
			t.Errorf("json missing %s: %s", key, text)
		}
	}
	if strings.Contains(text, `"status"`) {
		t.Errorf("unset status should be omitted: %s", text)
	}
}

func TestSpecOps(t *testing.T) {
	spec := New(&platform.ServiceContext{})
	if spec.Namespace != "bangumi" || spec.Label != "Bangumi" ||
		len(spec.Tags) != 1 || spec.Tags[0] != "media.metadata" {
		t.Errorf("spec = %+v", spec)
	}
	names := make([]string, 0, len(spec.Ops))
	for name := range spec.Ops {
		names = append(names, name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "fetchEpisodes,fetchMetadata,search" {
		t.Errorf("ops = %v", names)
	}
	result, err := spec.Ops["search"].Invoke(context.Background(), platform.RawJSON(`{"keyword":"  "}`))
	if err != nil {
		t.Fatal(err)
	}
	if results, ok := result.([]platform.SearchResult); !ok || len(results) != 0 {
		t.Errorf("blank search result = %+v", result)
	}
}
