// 豆瓣 provider:网络层 + MetadataProvider 实现。只支持按 ID 刮削(豆瓣没有稳定的
// 搜索 API);剧集请求 /movie/{id} 由上游 301 到 /v2/tv/{id},Go 客户端默认跟随。
// 必须带 Referer: https://movie.douban.com/,否则 invalid_request。

package douban

import (
	"context"
	"errors"

	"mygo-host/platform"
)

const apiBase = "https://m.douban.com/rexxar/api/v2"

const referer = "https://movie.douban.com/"

type provider struct {
	sc *platform.ServiceContext
}

func newProvider(sc *platform.ServiceContext) *provider {
	return &provider{sc: sc}
}

// Source 渠道身份。
func (p *provider) Source() string { return "douban" }

// Search 豆瓣没有稳定的搜索 API,只能按 ID 刮削(与 TS 一致直接报错)。
func (p *provider) Search(ctx context.Context, keyword string, mediaType platform.MediaType) ([]platform.SearchResult, error) {
	return nil, errors.New("[douban] 豆瓣没有稳定的搜索 API，只能按 ID 刮削")
}

// FetchMetadata 按 ID 抓取条目(真实媒体类型由上游 is_tv/episodes_count 推断)。
func (p *provider) FetchMetadata(ctx context.Context, id string, mediaType platform.MediaType) (*platform.MediaMetadata, error) {
	raw, err := p.sc.Cache.GetJSONCached(ctx, p.sc.HTTP, "douban",
		[]string{"douban", "subject", id}, apiBase+"/movie/"+id,
		map[string]string{"Referer": referer})
	if err != nil {
		return nil, err
	}
	return mapSubject(raw, id)
}

// FetchEpisodes 豆瓣没有每集接口,返回空数组。
func (p *provider) FetchEpisodes(ctx context.Context, meta *platform.MediaMetadata, id string) ([]platform.EpisodeMetadata, error) {
	return []platform.EpisodeMetadata{}, nil
}
