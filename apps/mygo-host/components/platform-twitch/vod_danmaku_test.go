// twitch VOD 弹幕测试:评论分页夹具(vod_test.go 的 /gql 复用)+ 流式收集。

package twitch

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"mygo-host/platform"
)

func writeCommentsPage(w http.ResponseWriter, extensions, variables map[string]any) {
	persisted, _ := extensions["persistedQuery"].(map[string]any)
	if persisted["sha256Hash"] != videoCommentsHash {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": "wrong hash"}}})
		return
	}
	cursor, _ := variables["cursor"].(string)
	videoID, _ := variables["videoID"].(string)
	if videoID == "closedchat" {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"video": map[string]any{"comments": nil},
		}})
		return
	}
	if videoID == "repeatcursor" {
		_ = json.NewEncoder(w).Encode(commentsEnvelope(
			`[{"cursor":"same","node":{"commenter":{"displayName":"a"},"contentOffsetSeconds":1,
			"message":{"fragments":[{"text":"hi"}]}}}]`, "same", true))
		return
	}
	switch cursor {
	case "":
		_ = json.NewEncoder(w).Encode(commentsEnvelope(
			`[{"cursor":"k1","node":{"commenter":{"login":"alice","displayName":"Alice"},
			"contentOffsetSeconds":28.5,"message":{"fragments":[{"text":"hello "},{"text":"world"}],"userColor":"#1e90ff"}}},
			{"cursor":"k1","node":{"commenter":null,"contentOffsetSeconds":30,
			"message":{"fragments":[{"text":""},{"text":"   "}]}}}]`, "k1", true))
	case "k1":
		_ = json.NewEncoder(w).Encode(commentsEnvelope(
			`[{"cursor":"k2","node":{"commenter":{"displayName":"Bob"},
			"contentOffsetSeconds":60,"message":{"fragments":[{"text":"gg"}],"userColor":"bad"}}}]`, "k2", false))
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}
}

func commentsEnvelope(edges, cursor string, hasNext bool) map[string]any {
	return map[string]any{"data": map[string]any{"video": map[string]any{"comments": map[string]any{
		"edges":    json.RawMessage(edges),
		"pageInfo": map[string]any{"hasNextPage": hasNext, "endCursor": cursor},
	}}}}
}

// collectDanmaku 订阅弹幕流并收集批次,直到条数达标或超时。
func collectDanmaku(t *testing.T, stream platform.DanmakuStream, want int) []platform.DanmakuItem {
	t.Helper()
	var mu sync.Mutex
	var got []platform.DanmakuItem
	done := make(chan struct{})
	cancel := stream(func(items []platform.DanmakuItem) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, items...)
		if len(got) >= want {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})
	t.Cleanup(cancel)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	mu.Lock()
	defer mu.Unlock()
	return got
}

func TestVODDanmakuPaging(t *testing.T) {
	sc := startVODServer(t)
	got := collectDanmaku(t, vodDanmakuStream(sc, vodGoodID), 2)
	if len(got) != 2 {
		t.Fatalf("items = %d, want 2(空文本/纯空白跳过)", len(got))
	}
	first := got[0]
	checkStr(t, "text", first.Text, "hello world")
	checkStrPtr(t, "user", first.User, "Alice")
	checkStrPtr(t, "color", first.Color, "#1E90FF")
	if first.TimeMs == nil || *first.TimeMs != 28500 {
		t.Errorf("timeMs = %v, want 28500", first.TimeMs)
	}
	second := got[1]
	checkStrPtr(t, "user2", second.User, "Bob")
	if second.Color != nil {
		t.Errorf("非法颜色应省略, got %v", *second.Color)
	}
	if second.TimeMs == nil || *second.TimeMs != 60000 {
		t.Errorf("timeMs2 = %v, want 60000", second.TimeMs)
	}
}

func TestVODDanmakuClosedChat(t *testing.T) {
	sc := startVODServer(t)
	got := collectDanmaku(t, vodDanmakuStream(sc, "closedchat"), 1)
	if len(got) != 0 {
		t.Errorf("关闭聊天应无条目, got %d", len(got))
	}
}

func TestVODDanmakuRepeatCursorStops(t *testing.T) {
	sc := startVODServer(t)
	got := collectDanmaku(t, vodDanmakuStream(sc, "repeatcursor"), 1)
	// 上游重复发同一 cursor 且 hasNextPage=true:游标不前进必须终止(只拿到 1 页 1 条)。
	if len(got) != 1 {
		t.Fatalf("items = %d, want 1(重复游标终止)", len(got))
	}
}

func TestDanmakuOpDefaultKind(t *testing.T) {
	spec := New(startVODServer(t))
	result, err := spec.Ops["danmaku"].Invoke(context.Background(), json.RawMessage(`{"id":"`+vodGoodID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	op := result.(platform.DanmakuOpResult)
	if op.Kind != "" || op.ID != vodGoodID {
		t.Errorf("danmaku op = %+v", op)
	}
	if op.Subscribe == nil {
		t.Fatal("subscribe 缺失")
	}
}
