// provider 网络/协议测试:用桩 transport 校验 404 → 空集、POST 搜索请求形状
// 与缓存命中,不发真实网络请求。

package bangumi

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"mygo-host/platform"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newStubProvider(t *testing.T, transport roundTripFunc) *provider {
	t.Helper()
	cache, err := platform.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := platform.NewHostClient()
	client.HTTP = &http.Client{Transport: transport}
	return newProvider(&platform.ServiceContext{
		Logf:  func(string, ...any) {},
		HTTP:  client,
		Cache: cache,
	})
}

func stubResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFetchEpisodesNotFoundReturnsEmpty(t *testing.T) {
	p := newStubProvider(t, func(*http.Request) (*http.Response, error) {
		return stubResponse(http.StatusNotFound, `{"title":"Not Found"}`), nil
	})
	eps, err := p.FetchEpisodes(context.Background(), nil, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 0 {
		t.Errorf("episodes = %+v", eps)
	}
}

func TestSearchBlankKeyword(t *testing.T) {
	p := newStubProvider(t, func(*http.Request) (*http.Response, error) {
		t.Error("blank keyword should not hit network")
		return stubResponse(http.StatusInternalServerError, ""), nil
	})
	results, err := p.Search(context.Background(), "   ", platform.MediaTV)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v", results)
	}
}

func TestSearchRequestShape(t *testing.T) {
	var gotMethod, gotURL, gotBody, gotUA string
	p := newStubProvider(t, func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		gotUA = r.Header.Get("User-Agent")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		return stubResponse(http.StatusOK, `{"data":[]}`), nil
	})
	results, err := p.Search(context.Background(), "命运", platform.MediaTV)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("results = %+v", results)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q", gotMethod)
	}
	if gotURL != "https://api.bgm.tv/v0/search/subjects?limit=20" {
		t.Errorf("url = %q", gotURL)
	}
	if gotBody != `{"keyword":"命运","filter":{"type":[2,6]}}` {
		t.Errorf("body = %s", gotBody)
	}
	if !strings.Contains(gotUA, "Mozilla") {
		t.Errorf("user-agent = %q", gotUA)
	}
}

func TestFetchMetadataCachesResponse(t *testing.T) {
	var gotURL string
	calls := 0
	body, err := os.ReadFile("testdata/bangumi_subject_8.json")
	if err != nil {
		t.Fatal(err)
	}
	p := newStubProvider(t, func(r *http.Request) (*http.Response, error) {
		calls++
		gotURL = r.URL.String()
		return stubResponse(http.StatusOK, string(body)), nil
	})
	for i := 0; i < 2; i++ {
		m, err := p.FetchMetadata(context.Background(), "8", platform.MediaTV)
		if err != nil {
			t.Fatal(err)
		}
		if m.Title != "Code Geass 反叛的鲁路修R2" {
			t.Errorf("title = %q", m.Title)
		}
	}
	if gotURL != "https://api.bgm.tv/v0/subjects/8" {
		t.Errorf("url = %q", gotURL)
	}
	if calls != 1 {
		t.Errorf("network calls = %d, want 1 (second hit should be cached)", calls)
	}
}
