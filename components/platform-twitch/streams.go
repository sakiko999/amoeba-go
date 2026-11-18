// 游标分页 source 装配(热门/分区直播与 VOD 回放共用):fetch = 首页;
// fetchMore = 游标翻页。core 首次翻页不传首页游标,故首次 fetchMore 先取一次
// 首页拿 endCursor 再取下一页。node→Item 映射在各渠道的 fetch 闭包内完成
// (sourceID/fetchedAt 是渠道语义)。

package twitch

import (
	"context"

	"mygo-host/platform"
)

// twitchItemsPage 已映射的游标页(条目 + 翻页信息)。
type twitchItemsPage struct {
	Items     []platform.Item
	Total     *int
	HasMore   bool
	EndCursor *string
}

func (p *twitchItemsPage) pageItems() []platform.Item { return p.Items }
func (p *twitchItemsPage) pageHasMore() bool          { return p.HasMore }
func (p *twitchItemsPage) pageEndCursor() *string     { return p.EndCursor }
func (p *twitchItemsPage) pageTotal() *int            { return p.Total }

// cursorPage fetch 闭包的返回契约。
type cursorPage interface {
	pageItems() []platform.Item
	pageHasMore() bool
	pageEndCursor() *string
	pageTotal() *int
}

// cursorSource 游标分页数据源(fetch/fetchMore)。
type cursorSource struct {
	sc        *platform.ServiceContext
	sourceID  string
	fetchPage func(ctx context.Context, after string) (cursorPage, error)
}

func (s *cursorSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	page, err := s.fetchPage(ctx, "")
	if err != nil {
		s.logFetchError(err)
		return platform.FetchResult{}, err
	}
	return platform.FetchResult{Items: page.pageItems(), Total: page.pageTotal()}, nil
}

func (s *cursorSource) FetchMore(ctx context.Context, cursor string, _ int) (platform.FetchPage, error) {
	page, err := s.pageFor(ctx, cursor)
	if err != nil {
		s.logFetchError(err)
		return platform.FetchPage{}, err
	}
	result := platform.FetchPage{Items: page.pageItems(), Total: page.pageTotal()}
	if page.pageHasMore() && page.pageEndCursor() != nil {
		result.Cursor = page.pageEndCursor()
	}
	return result, nil
}

// pageFor 有游标直接翻页;无游标(首次翻页)先取首页 endCursor,首页已到底则空页。
func (s *cursorSource) pageFor(ctx context.Context, cursor string) (cursorPage, error) {
	if cursor != "" {
		return s.fetchPage(ctx, cursor)
	}
	first, err := s.fetchPage(ctx, "")
	if err != nil {
		return nil, err
	}
	if !first.pageHasMore() || first.pageEndCursor() == nil {
		return &twitchItemsPage{Items: []platform.Item{}}, nil
	}
	return s.fetchPage(ctx, *first.pageEndCursor())
}

func (s *cursorSource) logFetchError(err error) {
	s.sc.Log("twitch 抓取失败(%s): %v", s.sourceID, err)
}

// streamItemsPage 直播流列表页 → 已映射游标页。
func streamItemsPage(page *twitchStreamPage, sourceID string, fetchedAt int64) *twitchItemsPage {
	items := make([]platform.Item, 0, len(page.Streams))
	for _, stream := range page.Streams {
		if item := twitchStreamToLive(stream, sourceID, fetchedAt); item != nil {
			items = append(items, *item)
		}
	}
	return &twitchItemsPage{Items: items, HasMore: page.HasMore, EndCursor: page.EndCursor}
}
