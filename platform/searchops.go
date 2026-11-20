// 搜索协议(searchOps):聚合搜索页的消费契约,scope ∈ {live(搜直播间),
// entity(搜实体:主播/频道/播客)}。平台按能力声明 scope;返回复用 FetchPage
// (当前只填 items,分页字段留给「加载更多」)。
// 平台统一搜索 op 契约：scope 校验、参数默认值与结果分页包装。

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SearchScope 搜索范围。
type SearchScope string

const (
	SearchLive   SearchScope = "live"
	SearchEntity SearchScope = "entity"
)

// defaultSearchPageSize 页大小缺省(平台端点另有上限的自行钳制)。
const defaultSearchPageSize = 20

// SearchParams search op 参数。
type SearchParams struct {
	Keyword  string      `json:"keyword"`
	Scope    SearchScope `json:"scope"`
	Page     int         `json:"page,omitempty"`     // 从 1 起,默认 1
	PageSize int         `json:"pageSize,omitempty"` // 默认 20
}

// SearchSpec 平台搜索能力声明。
type SearchSpec struct {
	Scopes []SearchScope
	Search func(ctx context.Context, p SearchParams) (FetchPage, error)
}

// SearchOps 装配 search op:keyword 必填、scope 必须在声明范围内、
// page/pageSize 补默认值、结果 Items 非 nil(JSON 输出 [] 而非 null)。
func SearchOps(spec SearchSpec) Ops {
	return Ops{
		"search": {
			Description: "按关键词搜索(scope: live=搜直播间, entity=搜实体)",
			Invoke: func(ctx context.Context, params json.RawMessage) (any, error) {
				var p SearchParams
				if err := DecodeParams(params, &p); err != nil {
					return nil, err
				}
				p.Keyword = strings.TrimSpace(p.Keyword)
				if p.Keyword == "" {
					return nil, fmt.Errorf("search 需要 keyword 参数")
				}
				if !scopeSupported(spec.Scopes, p.Scope) {
					return nil, fmt.Errorf("search 不支持 scope %q(可用:%s)", p.Scope, joinScopes(spec.Scopes))
				}
				if p.Page < 1 {
					p.Page = 1
				}
				if p.PageSize < 1 {
					p.PageSize = defaultSearchPageSize
				}
				result, err := spec.Search(ctx, p)
				if err != nil {
					return nil, err
				}
				if result.Items == nil {
					result.Items = []Item{}
				}
				return result, nil
			},
		},
	}
}

func scopeSupported(scopes []SearchScope, scope SearchScope) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func joinScopes(scopes []SearchScope) string {
	parts := make([]string, 0, len(scopes))
	for _, s := range scopes {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}

// SearchTags scope → 网关发现 tag(search.live / search.entity),
// 与 platform.data 一起进 Spec.Tags。
func SearchTags(scopes ...SearchScope) []string {
	tags := make([]string, 0, len(scopes))
	for _, s := range scopes {
		tags = append(tags, "search."+string(s))
	}
	return tags
}

// MergeOps 合并两组 ops(extra 覆盖同名项);平台在 FeedOps 之上追加私有 op 用。
func MergeOps(base, extra Ops) Ops {
	merged := make(Ops, len(base)+len(extra))
	for name, op := range base {
		merged[name] = op
	}
	for name, op := range extra {
		merged[name] = op
	}
	return merged
}
