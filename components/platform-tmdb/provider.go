// TMDB 网络层 + MetadataProvider 实现(凭证优先读 credentials.json 的 tmdb 项,
// 未设置时回退 TMDB_TOKEN 环境变量)。

package tmdb

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"mygo-host/platform"
)

type provider struct {
	sc *platform.ServiceContext
}

const (
	api  = "https://api.themoviedb.org/3"
	img  = "https://image.tmdb.org/t/p"
	lang = "zh-CN"
)

func newProvider(sc *platform.ServiceContext) *provider {
	return &provider{sc: sc}
}

func (p *provider) Source() string { return "tmdb" }

func (p *provider) authHeaders() (map[string]string, error) {
	token := ""
	if p.sc != nil && p.sc.Creds != nil {
		token, _ = p.sc.Creds.Lookup(KeyName)
	}
	if token == "" {
		token = os.Getenv("TMDB_TOKEN")
	}
	if token == "" {
		return nil, &platform.ProviderError{
			Source:  "tmdb",
			Message: "未设置 TMDB Token(凭据页或 TMDB_TOKEN 环境变量,v4 Read Access Token)。",
		}
	}
	return map[string]string{"Authorization": "Bearer " + token}, nil
}

func (p *provider) get(ctx context.Context, path string, keyParts []string) (platform.RawJSON, error) {
	headers, err := p.authHeaders()
	if err != nil {
		return nil, err
	}
	return p.sc.Cache.GetJSONCached(ctx, p.sc.HTTP, "tmdb", keyParts, api+path, headers)
}

func (p *provider) Search(ctx context.Context, keyword string, mediaType platform.MediaType) ([]platform.SearchResult, error) {
	if keyword == "" {
		return []platform.SearchResult{}, nil
	}
	path := "/search/movie"
	if mediaType == platform.MediaTV {
		path = "/search/tv"
	}
	raw, err := p.get(ctx, fmt.Sprintf("%s?language=%s&query=%s", path, lang, url.QueryEscape(keyword)),
		[]string{"tmdb", "search", string(mediaType), keyword})
	if err != nil {
		return nil, err
	}
	return mapSearch(raw, mediaType, 20)
}

func (p *provider) FetchMetadata(ctx context.Context, id string, mediaType platform.MediaType) (*platform.MediaMetadata, error) {
	path := fmt.Sprintf("/movie/%s", id)
	if mediaType == platform.MediaTV {
		path = fmt.Sprintf("/tv/%s", id)
	}
	raw, err := p.get(ctx, fmt.Sprintf("%s?language=%s&append_to_response=credits,external_ids", path, lang),
		[]string{"tmdb", string(mediaType), id})
	if err != nil {
		return nil, err
	}
	if mediaType == platform.MediaTV {
		return mapTvDetail(raw, id)
	}
	return mapMovieDetail(raw, id)
}

// FetchEpisodes 按季逐集抓取(N 季 = N 次请求);任一季失败只跳过。
func (p *provider) FetchEpisodes(ctx context.Context, meta *platform.MediaMetadata, id string) ([]platform.EpisodeMetadata, error) {
	if meta == nil || meta.SeasonCount == nil || *meta.SeasonCount <= 0 {
		return []platform.EpisodeMetadata{}, nil
	}
	all := []platform.EpisodeMetadata{}
	for s := 1; s <= *meta.SeasonCount; s++ {
		raw, err := p.get(ctx, fmt.Sprintf("/tv/%s/season/%d?language=%s", id, s, lang),
			[]string{"tmdb", "season", id, fmt.Sprintf("%d", s)})
		if err != nil {
			p.sc.Log("tmdb season %d fetch failed for %s: %v", s, id, err)
			continue
		}
		episodes, err := mapSeasonEpisodes(raw, s)
		if err != nil {
			continue
		}
		all = append(all, episodes...)
	}
	return all, nil
}
