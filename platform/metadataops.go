// metadata 协议(metadataOps 的 Go 版):search / fetchMetadata / fetchEpisodes,
// media-library 按命名空间统一调用(按 media.metadata tag 惰性发现渠道)。

package platform

import (
	"context"
	"encoding/json"
)

// MetadataProvider 媒体元数据 provider 契约(对齐 platform-contracts/provider.ts)。
type MetadataProvider interface {
	// Source 渠道身份(如 "tmdb")。
	Source() string
	Search(ctx context.Context, keyword string, mediaType MediaType) ([]SearchResult, error)
	FetchMetadata(ctx context.Context, id string, mediaType MediaType) (*MediaMetadata, error)
	// FetchEpisodes tv 每集元数据;不支持返回空。
	FetchEpisodes(ctx context.Context, meta *MediaMetadata, id string) ([]EpisodeMetadata, error)
}

type searchParams struct {
	Keyword   string    `json:"keyword"`
	MediaType MediaType `json:"mediaType"`
}

type fetchMetadataParams struct {
	ID        string    `json:"id"`
	MediaType MediaType `json:"mediaType"`
}

type fetchEpisodesParams struct {
	Meta *MediaMetadata `json:"meta"`
	ID   string         `json:"id"`
}

// MetadataOps 装配 metadata 协议 ops。
func MetadataOps(provider MetadataProvider) Ops {
	return Ops{
		"search": {
			Description: "按关键词搜索条目",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				var p searchParams
				if err := DecodeParams(params, &p); err != nil {
					return nil, err
				}
				mediaType := p.MediaType
				if mediaType == "" {
					mediaType = MediaTV
				}
				return provider.Search(ctx, p.Keyword, mediaType)
			},
		},
		"fetchMetadata": {
			Description: "按平台条目 id 取完整元数据",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				var p fetchMetadataParams
				if err := DecodeParams(params, &p); err != nil {
					return nil, err
				}
				mediaType := p.MediaType
				if mediaType == "" {
					mediaType = MediaTV
				}
				return provider.FetchMetadata(ctx, p.ID, mediaType)
			},
		},
		"fetchEpisodes": {
			Description: "取剧集每集元数据(不支持的平台返回空数组)",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				var p fetchEpisodesParams
				if err := DecodeParams(params, &p); err != nil {
					return nil, err
				}
				return provider.FetchEpisodes(ctx, p.Meta, p.ID)
			},
		},
	}
}
