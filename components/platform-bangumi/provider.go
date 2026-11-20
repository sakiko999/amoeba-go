// Bangumi 网络层 + MetadataProvider 实现:v0 读接口免 token 但必须带 UA
// (HostClient 自动补 DefaultUA);搜索是 POST + JSON body(带缓存),
// 详情/剧集 GET 带缓存;条目不存在(404)的剧集按空集处理。

package bangumi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"mygo-host/platform"
)

const (
	apiBase     = "https://api.bgm.tv/v0"
	searchLimit = 20
)

type provider struct {
	sc *platform.ServiceContext
}

type searchFilter struct {
	Type []int `json:"type"`
}

type searchBody struct {
	Keyword string       `json:"keyword"`
	Filter  searchFilter `json:"filter"`
}

func newProvider(sc *platform.ServiceContext) *provider {
	return &provider{sc: sc}
}

func (p *provider) Source() string { return "bangumi" }

// headers 带上凭据页保存的 Access Token(可选):v0 读接口免 token,带 token
// 只提升配额;未设置时不发 Authorization。
func (p *provider) headers() map[string]string {
	if p.sc == nil || p.sc.Creds == nil {
		return nil
	}
	if token, ok := p.sc.Creds.Lookup(KeyName); ok && token != "" {
		return map[string]string{"Authorization": "Bearer " + token}
	}
	return nil
}

func (p *provider) Search(ctx context.Context, keyword string, _ platform.MediaType) ([]platform.SearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return []platform.SearchResult{}, nil
	}
	body := searchBody{
		Keyword: keyword,
		Filter:  searchFilter{Type: []int{subjectTypeAnime, subjectTypeReal}},
	}
	url := fmt.Sprintf("%s/search/subjects?limit=%d", apiBase, searchLimit)
	raw, err := p.sc.Cache.PostJSONCached(ctx, p.sc.HTTP, "bangumi",
		[]string{"bangumi", "search", keyword, strconv.Itoa(searchLimit)}, url, body, p.headers())
	if err != nil {
		return nil, err
	}
	return mapSearch(raw, searchLimit)
}

func (p *provider) FetchMetadata(ctx context.Context, id string, mediaType platform.MediaType) (*platform.MediaMetadata, error) {
	raw, err := p.sc.Cache.GetJSONCached(ctx, p.sc.HTTP, "bangumi",
		[]string{"bangumi", "subject", id}, apiBase+"/subjects/"+id, p.headers())
	if err != nil {
		return nil, err
	}
	return mapSubject(raw, id, mediaType)
}

func (p *provider) FetchEpisodes(ctx context.Context, _ *platform.MediaMetadata, id string) ([]platform.EpisodeMetadata, error) {
	raw, err := p.sc.Cache.GetJSONCached(ctx, p.sc.HTTP, "bangumi",
		[]string{"bangumi", "episodes", id}, apiBase+"/episodes?subject_id="+id+"&type=0", nil)
	if err != nil {
		var notFound *platform.NotFoundError
		if errors.As(err, &notFound) {
			return []platform.EpisodeMetadata{}, nil
		}
		return nil, err
	}
	return mapEpisodes(raw, id)
}
