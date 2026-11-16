// 直播聊天轮询退避自检(移植 src/example/youtube-danmaku-poll.check.ts):
// 前两次沿用服务端 timeoutMs,连续失败指数退避封顶 60s;只有 continuations 用尽才终止。

package youtube

import "testing"

func TestNextPollDelay(t *testing.T) {
	cases := []struct {
		timeoutMs, failures, want int
	}{
		{5000, 0, 5000},
		{5000, 1, 5000},
		{5000, 2, 10000},
		{5000, 3, 20000},
		{5000, 5, 60000},
		{20, 3, 80},
		{5000, 100, 60000},
	}
	for _, c := range cases {
		if got := nextPollDelay(c.timeoutMs, c.failures); got != c.want {
			t.Errorf("nextPollDelay(%d, %d) = %d, want %d", c.timeoutMs, c.failures, got, c.want)
		}
	}
}

func TestIsChatEndedError(t *testing.T) {
	cases := map[string]bool{
		"youtube live chat: 直播结束(无下一 continuation)": true,
		"youtube live chat: 无 continuation(直播结束?)":  false,
		"网络中断": false,
	}
	for message, want := range cases {
		if got := isChatEndedError(message); got != want {
			t.Errorf("isChatEndedError(%q) = %v, want %v", message, got, want)
		}
	}
}
