// 移植 TS example/danmaku-proto.check.ts：截断/畸形帧必须返回而非死循环 + 正常帧解码。

package bili

import (
	"testing"
	"time"
)

// guardDecode 守护死循环（修复前截断帧会挂死整个进程）。
func guardDecode(t *testing.T, buf []byte) []danmakuElem {
	t.Helper()
	done := make(chan []danmakuElem, 1)
	go func() { done <- decodeDanmakuSeg(buf) }()
	select {
	case result := <-done:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("decodeDanmakuSeg 未返回（死循环）")
		return nil
	}
}

func TestDecodeDanmakuSeg(t *testing.T) {
	// 长度声明超出缓冲的截断帧。
	if result := guardDecode(t, []byte{0x0a, 0x64, 0x10, 0x01, 0x18}); result == nil {
		t.Error("截断帧应返回空数组而非 nil")
	}
	// 未知 wire(group 3) 必须不卡。
	if result := guardDecode(t, []byte{0x0b, 0x0c}); result == nil {
		t.Error("group-wire 应返回空数组")
	}
	// 正常帧：progress=100 / mode=1 / content=ABCD / 默认白色。
	good := guardDecode(t, []byte{0x0a, 0x0b, 0x10, 0x64, 0x18, 0x01, 0x3a, 0x04, 0x41, 0x42, 0x43, 0x44})
	if len(good) != 1 || good[0].content != "ABCD" || good[0].progress != 100 || good[0].mode != 1 || good[0].color != 0xffffff {
		t.Errorf("正常帧解码 = %+v", good)
	}
	if result := guardDecode(t, nil); len(result) != 0 {
		t.Errorf("空缓冲 = %+v", result)
	}
}

func TestArgbToHex(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0xffffff, "#ffffff"},
		{0xff0000, "#ff0000"},
		{0x00ff00, "#00ff00"},
		{0xffffffff, "#ffffff"},
		{0x1, "#ffffff"},
	}
	for _, c := range cases {
		if got := argbToHex(c.in); got != c.want {
			t.Errorf("argbToHex(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
