// search 结果的进程内 TTL 缓存:同 keyword+platforms 的查询在 TTL 内
// 直接复用上次快照(页面重挂载 / 返回再搜不重复打平台)。只缓存全部平台
// 成功的快照——带失败组的结果是暂态,缓存会让一次抖动粘住整个 TTL。
// 容量按插入序 FIFO 逐出,防关键词无限膨胀。

package search

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// resultTTL 缓存有效期(var:测试可调小)。
var resultTTL = 5 * time.Minute

// maxEntries 缓存条目上限;超出后逐出最旧插入的条目。
const maxEntries = 32

type cache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	order   []string // 插入序(重复 key 不重复入列)
}

type cacheEntry struct {
	result  Result
	expires time.Time
}

func newCache() *cache { return &cache{entries: map[string]cacheEntry{}} }

// cacheKey 归一化查询标识:keyword 小写、platforms 排序,使参数顺序不影响命中。
func cacheKey(p QueryParams) string {
	platforms := append([]string(nil), p.Platforms...)
	sort.Strings(platforms)
	return strings.ToLower(p.Keyword) + "|" + strings.Join(platforms, ",")
}

func (c *cache) get(key string, now time.Time) (Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || !now.Before(e.expires) {
		return Result{}, false
	}
	return e.result, true
}

func (c *cache) store(key string, r Result, now time.Time) {
	for _, g := range r.Groups {
		if g.Error != nil {
			return
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = cacheEntry{result: r, expires: now.Add(resultTTL)}
	for len(c.order) > maxEntries {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}
