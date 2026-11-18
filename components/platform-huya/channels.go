// 保留类渠道实现(非单端点同形,不走 MakeChannel):
// 房间 channel 多房间并发聚合、单房间失败隔离;分区列表 channel 多顶层分区聚合、
// 单分区失败隔离,全部为空才报错(不能静默给空目录)。

package huya

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"mygo-host/platform"
)

// liveChannel 虎牙直播房间 channel(roomIds 逗号分隔,可多个)。
type liveChannel struct{ sc *platform.ServiceContext }

func newLiveChannel(sc *platform.ServiceContext) *liveChannel { return &liveChannel{sc: sc} }

func (c *liveChannel) Key() string  { return "live:huya" }
func (c *liveChannel) Name() string { return "虎牙直播房间" }
func (c *liveChannel) Kind() platform.Kind {
	return platform.KindLive
}

func (c *liveChannel) SourceInfoTpl() []platform.SourceInfoField {
	return []platform.SourceInfoField{{
		Key:      "roomIds",
		Label:    "直播间 ID(逗号分隔,可多个)",
		Required: platform.Bool(true),
	}}
}

func (c *liveChannel) DefaultInfo() platform.SourceInfo { return nil }

func (c *liveChannel) GetSource(info platform.SourceInfo) platform.DataSource {
	return &liveSource{sc: c.sc, info: info}
}

type liveSource struct {
	sc   *platform.ServiceContext
	info platform.SourceInfo
}

var roomIDRe = regexp.MustCompile(`[,，\s]+`)

// parseRoomIDs 解析订阅信息里的房间列表(逗号/中文逗号/空白分隔)。
func parseRoomIDs(info platform.SourceInfo) []string {
	parts := roomIDRe.Split(info["roomIds"], -1)
	roomIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			roomIDs = append(roomIDs, part)
		}
	}
	return roomIDs
}

func (s *liveSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	roomIDs := parseRoomIDs(s.info)
	if len(roomIDs) == 0 {
		return platform.FetchResult{}, fmt.Errorf("live:huya 需要 roomIds")
	}
	t := s.sc.NowMs()
	results := make([]*platform.Item, len(roomIDs))
	var wg sync.WaitGroup
	for i, roomID := range roomIDs {
		wg.Add(1)
		go func(i int, roomID string) {
			defer wg.Done()
			data, err := fetchProfileRoom(ctx, s.sc, roomID)
			if err != nil {
				s.sc.Log("huya 房间 %s 拉取失败,跳过: %v", roomID, err)
				return
			}
			item := mapRoom(roomID, data, t)
			results[i] = &item
		}(i, roomID)
	}
	wg.Wait()
	items := make([]platform.Item, 0, len(roomIDs))
	for _, item := range results {
		if item != nil {
			items = append(items, *item)
		}
	}
	// 全部失败 ≠ 成功 0 条(离线房间仍返回 Live);静默空会清空消费方内容。
	if len(items) == 0 {
		return platform.FetchResult{}, fmt.Errorf("live:huya 全部房间拉取失败(%d 个)", len(roomIDs))
	}
	return platform.FetchResult{Items: items}, nil
}

// categoriesChannel 虎牙分区目录订阅(顶层 4 个固定枚举 → 二级分区)。
type categoriesChannel struct{ sc *platform.ServiceContext }

func newCategoriesChannel(sc *platform.ServiceContext) *categoriesChannel {
	return &categoriesChannel{sc: sc}
}

func (c *categoriesChannel) Key() string  { return "live:huya:categories" }
func (c *categoriesChannel) Name() string { return "虎牙分区列表" }
func (c *categoriesChannel) Kind() platform.Kind {
	return platform.KindCategory
}

func (c *categoriesChannel) SourceInfoTpl() []platform.SourceInfoField { return nil }

func (c *categoriesChannel) DefaultInfo() platform.SourceInfo { return platform.SourceInfo{} }

func (c *categoriesChannel) GetSource(_ platform.SourceInfo) platform.DataSource {
	return &categoriesSource{sc: c.sc}
}

type categoriesSource struct{ sc *platform.ServiceContext }

func (s *categoriesSource) Fetch(ctx context.Context) (platform.FetchResult, error) {
	groups := make([][]subCategory, len(topCategories))
	var wg sync.WaitGroup
	for i, top := range topCategories {
		wg.Add(1)
		go func(i int, parentID string) {
			defer wg.Done()
			subs, err := fetchSubCategories(ctx, s.sc, parentID)
			if err != nil {
				return // 单个顶层分区失败不影响其余
			}
			groups[i] = subs
		}(i, top.ID)
	}
	wg.Wait()
	subs := make([]subCategory, 0)
	for _, group := range groups {
		subs = append(subs, group...)
	}
	if len(subs) == 0 {
		return platform.FetchResult{}, fmt.Errorf("huya: 分区列表拉取失败")
	}
	return platform.FetchResult{Items: mapCategoryRows("live:huya:categories", s.sc.NowMs(), subs)}, nil
}
