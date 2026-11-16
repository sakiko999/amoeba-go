// 数据渠道契约(与 platform-contracts/src/channel.ts 对齐)+ makeApiChannel 装配
// (与 platform-kit/src/data.ts 对齐):「单端点 GET → 列表 → 映射」同形渠道的
// 声明式规格,数值游标分页样板收敛在此。

package platform

import "context"

// SourceInfo 渠道实例化参数(如 {uid} / {roomIds})。
type SourceInfo map[string]string

// SourceInfoField 渠道参数字段(供 UI 生成订阅表单)。
type SourceInfoField struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Required    *bool   `json:"required,omitempty"`
	Placeholder *string `json:"placeholder,omitempty"`
}

// FetchResult 一次抓取:条目 + 上游真实总数(可选)。
type FetchResult struct {
	Items []Item `json:"items"`
	Total *int   `json:"total,omitempty"`
}

// FetchPage 翻页结果:cursor 省略 = 没有更多。
type FetchPage struct {
	Items   []Item  `json:"items"`
	Total   *int    `json:"total,omitempty"`
	Cursor  *string `json:"cursor,omitempty"`
	HasMore *bool   `json:"hasMore,omitempty"`
}

// DataSource 实例化后的数据源(行为载体)。
type DataSource interface {
	Fetch(ctx context.Context) (FetchResult, error)
}

// PageableSource 翻页能力(可选):cursor 由上次返回给出;existing<0 表示消费方
// 未提供已持有条数(TS 侧 undefined)。
type PageableSource interface {
	FetchMore(ctx context.Context, cursor string, existing int) (FetchPage, error)
}

// PassiveSource 被动采集(浏览器渠道;第一阶段平台不实现,协议位保留)。
type PassiveSource interface {
	FetchPassive(ctx context.Context) (*FetchResult, error) // nil = 不可用
}

// DataChannel 一个数据渠道(纯描述;GetSource 实例化才承担抓取)。
type DataChannel interface {
	Key() string
	Name() string
	Kind() Kind
	// SourceInfoTpl 无参渠道返回 nil。
	SourceInfoTpl() []SourceInfoField
	// DefaultInfo 无默认实例返回 nil。
	DefaultInfo() SourceInfo
	GetSource(info SourceInfo) DataSource
}

// ChannelSpec 声明式渠道规格(makeApiChannel 的输入;对应 TS ApiChannelSpec)。
type ChannelSpec struct {
	Key         string
	Name        string
	Kind        Kind
	InfoTpl     []SourceInfoField
	DefaultInfo SourceInfo
	// FetchPage 取第 page 页条目(页码从 1 起);返回值二选一:纯列表或带
	// hasMore/total 的结构(用 PageOfItems / PageOf 包装)。
	FetchPage func(ctx context.Context, info SourceInfo, page int) (PageItems, error)
	// Pageable 数值翻页(省略 = 单页无翻页)。First:首页之后的起始页;Step:页步进。
	Pageable *PageableSpec
}

// PageableSpec 数值翻页参数。
type PageableSpec struct {
	First int
	Step  int
}

// PageItems fetchPage 的返回:列表 + 可选 hasMore/total。
type PageItems struct {
	Items   []Item
	HasMore *bool
	Total   *int
}

type apiChannel struct {
	spec ChannelSpec
}

func (c *apiChannel) Key() string                      { return c.spec.Key }
func (c *apiChannel) Name() string                     { return c.spec.Name }
func (c *apiChannel) Kind() Kind                       { return c.spec.Kind }
func (c *apiChannel) SourceInfoTpl() []SourceInfoField { return c.spec.InfoTpl }
func (c *apiChannel) DefaultInfo() SourceInfo          { return c.spec.DefaultInfo }

func (c *apiChannel) GetSource(info SourceInfo) DataSource {
	if c.spec.Pageable == nil {
		// 单页渠道不实现 PageableSource(消费方带 cursor 请求时,FeedOps 会
		// 按 TS 语义报「渠道 X 不支持翻页」)。
		return &apiFetchSource{spec: c.spec, info: info}
	}
	return &apiSource{spec: c.spec, info: info}
}

// apiFetchSource 单页数据源(无翻页能力)。
type apiFetchSource struct {
	spec ChannelSpec
	info SourceInfo
}

func (s *apiFetchSource) Fetch(ctx context.Context) (FetchResult, error) {
	page, err := s.spec.FetchPage(ctx, s.info, 1)
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{Items: page.Items, Total: page.Total}, nil
}

type apiSource struct {
	spec ChannelSpec
	info SourceInfo
}

func (s *apiSource) Fetch(ctx context.Context) (FetchResult, error) {
	page, err := s.spec.FetchPage(ctx, s.info, 1)
	if err != nil {
		return FetchResult{}, err
	}
	return FetchResult{Items: page.Items, Total: page.Total}, nil
}

// FetchMore 数值游标翻页(spec.Pageable 非 nil 才会挂到本类型上)。
func (s *apiSource) FetchMore(ctx context.Context, cursor string, existing int) (FetchPage, error) {
	p := s.spec.Pageable
	n := p.First
	if parsed, ok := atoiSafe(cursor); ok && cursor != "" {
		n = parsed
	}
	page, err := s.spec.FetchPage(ctx, s.info, n)
	if err != nil {
		return FetchPage{}, err
	}
	result := FetchPage{Items: page.Items, Total: page.Total}
	hasMore := page.HasMore
	if hasMore == nil && len(page.Items) > 0 {
		hasMore = Bool(true)
	}
	if hasMore != nil && *hasMore {
		result.Cursor = Str(itoa(n + p.Step))
	}
	if page.HasMore != nil {
		result.HasMore = page.HasMore
	}
	return result, nil
}

// MakeChannel 声明式装配(对应 TS makeApiChannel)。
func MakeChannel(spec ChannelSpec) DataChannel { return &apiChannel{spec: spec} }

func atoiSafe(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
