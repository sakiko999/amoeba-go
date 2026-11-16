// 异步 setup + build 组合成 DanmakuStream（对齐 platform-kit deferredStream）：
// 退订发生在 setup 完成前则不再建流（连接零泄漏）。

package bili

import (
	"sync"

	"mygo-host/platform"
)

func deferredStream[T any](
	setup func() (T, error),
	build func(T, func([]platform.DanmakuItem)) func(),
	onError func(error),
) platform.DanmakuStream {
	return func(onItems func([]platform.DanmakuItem)) func() {
		var (
			mu      sync.Mutex
			stopped bool
			unsub   func()
		)
		go func() {
			value, err := setup()
			if err != nil {
				if !isStopped(&mu, &stopped) && onError != nil {
					onError(err)
				}
				return
			}
			if isStopped(&mu, &stopped) {
				return
			}
			u := build(value, onItems)
			mu.Lock()
			unsub = u
			s := stopped
			mu.Unlock()
			if s && u != nil {
				u()
			}
		}()
		return func() {
			mu.Lock()
			stopped = true
			u := unsub
			mu.Unlock()
			if u != nil {
				u()
			}
		}
	}
}

func isStopped(mu *sync.Mutex, stopped *bool) bool {
	mu.Lock()
	defer mu.Unlock()
	return *stopped
}
