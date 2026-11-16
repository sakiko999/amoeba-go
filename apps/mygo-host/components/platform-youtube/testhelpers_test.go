// 测试公共辅助:ServiceContext、指针构造器、包级缓存重置(测试隔离)。

package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mygo-host/platform"
)

func testContext(t *testing.T) *platform.ServiceContext {
	t.Helper()
	return &platform.ServiceContext{
		Logf: func(string, ...any) {},
		HTTP: platform.NewHostClient(),
		JS: platform.JSRunnerFunc(func(ctx context.Context, code, fn string, args []any) (any, error) {
			return nil, errors.New("no js")
		}),
	}
}

func fptr(value float64) *float64 { return &value }

func iptr(value int) *int { return &value }

// resetPackageState 清空包级缓存(visitorData / base.js / 直播页),避免测试互相污染。
func resetPackageState() {
	visitorState.mu.Lock()
	visitorState.value, visitorState.err, visitorState.wait = "", nil, nil
	visitorState.mu.Unlock()

	playerCodeState.mu.Lock()
	playerCodeState.url, playerCodeState.code = "", ""
	playerCodeState.mu.Unlock()

	deobfState.mu.Lock()
	deobfState.cache = nil
	deobfState.mu.Unlock()

	liveShelvesState.mu.Lock()
	liveShelvesState.cache = nil
	liveShelvesState.mu.Unlock()
}

func mustJSON(t *testing.T, raw string) *jsonValue {
	t.Helper()
	value, err := parseJSONValue([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustMarshal(t *testing.T, value any) *jsonValue {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return mustJSON(t, string(raw))
}
