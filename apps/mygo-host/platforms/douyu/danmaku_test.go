// 弹幕帧测试(移植 TS example/douyu-frame.check.ts):
// 一个 WS 帧粘多个 STT 包时必须逐包全解,截断包不得死循环。

package douyu

import (
	"encoding/binary"
	"strconv"
	"testing"
)

// mkServerPacket 造一个 server→client(690)STT 包。
func mkServerPacket(stt string) []byte {
	body := []byte(stt)
	full := 9 + len(body)
	frame := make([]byte, full+4)
	binary.LittleEndian.PutUint32(frame[0:], uint32(full))
	binary.LittleEndian.PutUint32(frame[4:], uint32(full))
	binary.LittleEndian.PutUint16(frame[8:], 690)
	copy(frame[12:], body)
	return frame
}

func chatMessage(text, user string, col int) string {
	return "type@=chatmsg/txt@=" + text + "/nn@=" + user + "/col@=" + strconv.Itoa(col) + "/dms@=1/"
}

func TestDouyuFrameEncoding(t *testing.T) {
	frame := douyuFrame("type@=mrkl/")
	bodyLen := len("type@=mrkl/")
	if len(frame) != 12+bodyLen+1 {
		t.Fatalf("frame len = %d", len(frame))
	}
	if binary.LittleEndian.Uint32(frame[0:]) != uint32(9+bodyLen) ||
		binary.LittleEndian.Uint32(frame[4:]) != uint32(9+bodyLen) ||
		binary.LittleEndian.Uint16(frame[8:]) != 689 {
		t.Errorf("frame header = %x", frame[:12])
	}
	if frame[10] != 0 || frame[11] != 0 || frame[len(frame)-1] != 0 {
		t.Errorf("frame padding = %v", frame)
	}
}

func TestParseDouyuFrameSinglePacket(t *testing.T) {
	items := parseDouyuFrame(mkServerPacket(chatMessage("hello", "userA", 0)))
	if len(items) != 1 || items[0].Text != "hello" || *items[0].User != "userA" {
		t.Fatalf("single packet = %+v", items)
	}
	if items[0].Color != nil {
		t.Errorf("col=0 不应有颜色: %v", *items[0].Color)
	}
}

func TestParseDouyuFrameStickyPackets(t *testing.T) {
	p1 := mkServerPacket(chatMessage("第一条", "A", 0))
	p2 := mkServerPacket(chatMessage("第二条", "B", 1))
	p3 := mkServerPacket(chatMessage("第三条", "C", 0))
	merged := append(append(append([]byte{}, p1...), p2...), p3...)
	items := parseDouyuFrame(merged)
	if len(items) != 3 {
		t.Fatalf("粘包解出 %d 条, want 3", len(items))
	}
	if items[0].Text != "第一条" || items[1].Text != "第二条" || items[2].Text != "第三条" {
		t.Errorf("顺序/内容错误: %s,%s,%s", items[0].Text, items[1].Text, items[2].Text)
	}
	// 斗鱼专属 6 色索引:1 红、2 蓝。
	if items[1].Color == nil {
		t.Error("col=1 颜色缺失")
	} else if *items[1].Color != "#ff0000" {
		t.Errorf("col=1 颜色 = %q", *items[1].Color)
	}
}

func TestParseDouyuFrameSkipsNonChat(t *testing.T) {
	if items := parseDouyuFrame(mkServerPacket("type@=uenter/nn@=X/")); len(items) != 0 {
		t.Errorf("非 chatmsg 被解析: %+v", items)
	}
	if items := parseDouyuFrame(mkServerPacket("type@=chatmsg/txt@=no-dms/nn@=X/")); len(items) != 0 {
		t.Errorf("dms 缺失(禁言)未跳过: %+v", items)
	}
}

func TestDouyuColorMapping(t *testing.T) {
	cases := []struct {
		col  string
		want string
	}{
		{"1", "#ff0000"}, {"2", "#1e87f0"}, {"3", "#7ac84b"},
		{"4", "#ff7f00"}, {"5", "#9b39f4"}, {"6", "#ff69b4"},
	}
	for _, c := range cases {
		got := douyuColorOf(c.col)
		if got == nil || *got != c.want {
			t.Errorf("col %s = %v, want %s", c.col, got, c.want)
		}
	}
	for _, col := range []string{"0", "7", "abc", ""} {
		if got := douyuColorOf(col); got != nil {
			t.Errorf("col %q 不应有颜色: %v", col, *got)
		}
	}
}

func TestParseDouyuFrameTruncated(t *testing.T) {
	p1 := mkServerPacket(chatMessage("第一条", "A", 0))
	p2 := mkServerPacket(chatMessage("第二条", "B", 0))
	p3 := mkServerPacket(chatMessage("第三条", "C", 0))
	merged := append(append(append([]byte{}, p1...), p2...), p3...)
	items := parseDouyuFrame(merged[:len(merged)-3])
	if len(items) != 2 || items[0].Text != "第一条" || items[1].Text != "第二条" {
		t.Errorf("截断尾包 → 前两包仍解出, got %+v", items)
	}
}
