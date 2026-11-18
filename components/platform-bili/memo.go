// memo 按 key 缓存异步字符串结果：合集视频链接 → seasonId、系列 → 作者 mid。
// 失败不缓存（下次重试）；并发同 key 共享一次请求（对齐 TS 的模块级 Promise 缓存）。

package bili

import "sync"

type memo struct {
	mu sync.Mutex
	m  map[string]*memoEntry
}

type memoEntry struct {
	done  chan struct{}
	value string
	err   error
}

func (m *memo) get(key string, fetch func() (string, error)) (string, error) {
	m.mu.Lock()
	if m.m == nil {
		m.m = map[string]*memoEntry{}
	}
	if e, ok := m.m[key]; ok {
		m.mu.Unlock()
		<-e.done
		return e.value, e.err
	}
	e := &memoEntry{done: make(chan struct{})}
	m.m[key] = e
	m.mu.Unlock()

	e.value, e.err = fetch()

	m.mu.Lock()
	if e.err != nil {
		delete(m.m, key)
	}
	close(e.done)
	m.mu.Unlock()
	return e.value, e.err
}
