// Apple 目录网络访问:榜单 / Lookup / 搜索三条链路,均为普通 GET(显式 UA)。
// 端点与 TS catalog.ts 一致;Lookup 按每批 100 个 id 分批,守住 URL 长度。

package apple

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"mygo-host/platform"
)

const (
	itunesBase  = "https://itunes.apple.com"
	catalogUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	lookupBatch = 100
)

type catalog struct {
	sc *platform.ServiceContext
}

func (c *catalog) getJSON(ctx context.Context, requestURL string, out any) error {
	raw, err := c.sc.HTTP.GetJSON(ctx, requestURL, map[string]string{"user-agent": catalogUA})
	if err != nil {
		return err
	}
	return platform.UnmarshalJSON(raw, out)
}

func (c *catalog) chartEntries(ctx context.Context, country string, genreID, limit int) ([]chartPick, error) {
	requestURL := fmt.Sprintf("%s/%s/rss/toppodcasts/limit=%d/genre=%d/json", itunesBase, country, limit, genreID)
	raw, err := c.sc.HTTP.GetJSON(ctx, requestURL, map[string]string{"user-agent": catalogUA})
	if err != nil {
		return nil, err
	}
	return parseChart(raw)
}

// lookup 按 id 批量取详情(这一步才拿到 feedUrl)。
func (c *catalog) lookup(ctx context.Context, ids []string, country string) ([]podcast, error) {
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		if value := strings.TrimSpace(id); value != "" {
			clean = append(clean, value)
		}
	}
	out := []podcast{}
	for start := 0; start < len(clean); start += lookupBatch {
		end := start + lookupBatch
		if end > len(clean) {
			end = len(clean)
		}
		requestURL := fmt.Sprintf("%s/lookup?id=%s&entity=podcast&country=%s",
			itunesBase, strings.Join(clean[start:end], ","), country)
		var res lookupResponse
		if err := c.getJSON(ctx, requestURL, &res); err != nil {
			return nil, err
		}
		out = append(out, podcastsOf(res)...)
	}
	return out, nil
}

// chart 热门榜:先取榜单拿 id,再 Lookup 换 feedUrl(Apple 接口约束)。
func (c *catalog) chart(ctx context.Context, genreID, limit int, country string) ([]podcast, error) {
	entries, err := c.chartEntries(ctx, country, genreID, limit)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []podcast{}, nil
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.id)
	}
	found, err := c.lookup(ctx, ids, country)
	if err != nil {
		return nil, err
	}
	return mergeChart(entries, found), nil
}

// search 关键词搜节目(结果直接带 feedUrl,不需要二次 Lookup)。
func (c *catalog) search(ctx context.Context, term string, limit int, country string) ([]podcast, error) {
	keyword := strings.TrimSpace(term)
	if keyword == "" {
		return []podcast{}, nil
	}
	requestURL := fmt.Sprintf("%s/search?term=%s&media=podcast&entity=podcast&limit=%d&country=%s",
		itunesBase, encodeQuery(keyword), limit, country)
	var res lookupResponse
	if err := c.getJSON(ctx, requestURL, &res); err != nil {
		return nil, err
	}
	return podcastsOf(res), nil
}

// encodeQuery 对应 encodeURIComponent:空格输出 %20(其余转义语义等价)。
func encodeQuery(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
