// 测试共用:fixture 读取、固定时钟的 ServiceContext 与假 HTTP 传输。

package apple

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"mygo-host/platform"
)

const fixedNow = int64(1700000000000)

func fixture(t *testing.T, name string) platform.RawJSON {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testContext(t *testing.T, client *http.Client) *platform.ServiceContext {
	t.Helper()
	if client == nil {
		client = &http.Client{}
	}
	return &platform.ServiceContext{
		Logf: func(string, ...any) {},
		HTTP: &platform.HostClient{HTTP: client},
		Now:  func() int64 { return fixedNow },
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// fakeCatalogClient 按 URL 片段返回 fixture,记录请求 URL 并校验显式 UA。
func fakeCatalogClient(t *testing.T, routes map[string]platform.RawJSON) (*http.Client, *[]string) {
	t.Helper()
	urls := &[]string{}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*urls = append(*urls, req.URL.String())
		if ua := req.Header.Get("User-Agent"); ua != catalogUA {
			t.Errorf("UA = %q", ua)
		}
		for fragment, body := range routes {
			if strings.Contains(req.URL.String(), fragment) {
				return jsonResponse(string(body)), nil
			}
		}
		t.Errorf("unexpected request url %s", req.URL)
		return jsonResponse(`{}`), nil
	})}
	return client, urls
}

// fetchChartResult 走 fetch op 的 chart 渠道(假传输注入真实 fixture)。
func fetchChartResult(t *testing.T) (platform.FetchResult, []string) {
	t.Helper()
	client, urls := fakeCatalogClient(t, map[string]platform.RawJSON{
		"/lookup":           fixture(t, "lookup_cn_3.json"),
		"/rss/toppodcasts/": fixture(t, "chart_cn_genre26_limit3.json"),
	})
	spec := New(testContext(t, client))
	result, err := spec.Ops["fetch"].Invoke(context.Background(),
		json.RawMessage(`{"channel":"apple:podcast-chart","info":{"genre":"1324","limit":"3","country":"us"}}`))
	if err != nil {
		t.Fatal(err)
	}
	feed, ok := result.(platform.FetchResult)
	if !ok {
		t.Fatalf("fetch result type = %T", result)
	}
	return feed, *urls
}
