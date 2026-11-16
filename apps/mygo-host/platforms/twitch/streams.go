// 直播流列表 source 装配(热门 / 分区共用):fetch = 首页;fetchMore = 游标翻页。
// core 首次翻页不传首页游标,故首次 fetchMore 先取一次首页拿 endCursor 再取下一页。

package twitch

import (
	"context"

	"mygo-host/platform"
)

type streamsSource struct {
	sc        *platform.ServiceContext
	sourceID  string
	fetchPage func(ctx context.Context, after string) (*twitchStreamPage, error)
}

func (s *streamsSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	page, err := s.fetchPage(ctx, "")
	if err != nil {
		s.logFetchError(err)
		return platform.FetchResult{}, err
	}
	return platform.FetchResult{Items: s.toItems(page)}, nil
}

func (s *streamsSource) FetchMore(ctx context.Context, cursor string, _ int) (platform.FetchPage, error) {
	page, err := s.pageFor(ctx, cursor)
	if err != nil {
		s.logFetchError(err)
		return platform.FetchPage{}, err
	}
	items := s.toItems(page)
	result := platform.FetchPage{Items: items}
	if page.HasMore && page.EndCursor != nil {
		result.Cursor = page.EndCursor
	}
	return result, nil
}

// pageFor 有游标直接翻页;无游标(首次翻页)先取首页 endCursor,首页已到底则空结果。
func (s *streamsSource) pageFor(ctx context.Context, cursor string) (*twitchStreamPage, error) {
	if cursor != "" {
		return s.fetchPage(ctx, cursor)
	}
	first, err := s.fetchPage(ctx, "")
	if err != nil {
		return nil, err
	}
	if !first.HasMore || first.EndCursor == nil {
		return &twitchStreamPage{}, nil
	}
	return s.fetchPage(ctx, *first.EndCursor)
}

func (s *streamsSource) toItems(page *twitchStreamPage) []platform.Item {
	fetchedAt := s.sc.NowMs()
	items := make([]platform.Item, 0, len(page.Streams))
	for _, stream := range page.Streams {
		if item := twitchStreamToLive(stream, s.sourceID, fetchedAt); item != nil {
			items = append(items, *item)
		}
	}
	return items
}

func (s *streamsSource) logFetchError(err error) {
	s.sc.Log("twitch 抓取失败(%s): %v", s.sourceID, err)
}
