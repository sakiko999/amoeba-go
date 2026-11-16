// Tars 编解码黄金集:移植 example/huya-tars.check.ts 的四个用例 —— 正常帧可解、
// 截断帧不死循环、readStructAt 截断仍返回、ZERO_TAG 不误伤;另断言写入字节序
// (类型 tag 语义)与读取的 tag 定位行为。

package huya

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"mygo-host/platform"
)

func beginByte(tag int) byte { return byte(tag<<4 | tarsStructBegin) }
func endByte(tag int) byte   { return byte(tag<<4 | tarsStructEnd) }
func zeroByte(tag int) byte  { return byte(tag<<4 | tarsZeroTag) }

// hyFrame 包成完整帧:outer{tag0=7, tag1=push{tag1=1400(uri), tag2=HYMessage}}。
func hyFrame(message []byte) []byte {
	push := &tarsWriter{}
	push.writeInt(1400, 1)
	push.writeBytes(message, 2)
	outer := &tarsWriter{}
	outer.writeInt(7, 0)
	outer.writeBytes(push.bytes(), 1)
	return outer.bytes()
}

// hyMessage HYMessage:tag0=sender(内含 tag2 用户名)+ tag3 正文;
// withEnd=false 时去掉 sender 的 STRUCT_END(截断帧)。
func hyMessage(user, text string, withEnd bool) []byte {
	sender := &tarsWriter{}
	sender.writeString(user, 2)
	textField := &tarsWriter{}
	textField.writeString(text, 3)
	body := []byte{beginByte(0)}
	body = append(body, sender.bytes()...)
	if withEnd {
		body = append(body, endByte(0))
	}
	return append(body, textField.bytes()...)
}

func TestDecodeNormalFrame(t *testing.T) {
	items := decodeDanmakuFrame(hyFrame(hyMessage("abc", "hello", true)))
	if len(items) != 1 || items[0].Text != "hello" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].User == nil || *items[0].User != "abc" {
		t.Errorf("user = %v", items[0].User)
	}
	if items[0].Color != nil {
		t.Errorf("color = %v", items[0].Color)
	}
}

func TestDecodeTruncatedFrameReturns(t *testing.T) {
	done := make(chan []platform.DanmakuItem, 1)
	go func() {
		done <- decodeDanmakuFrame(hyFrame(hyMessage("abc", "hello", false)))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("截断帧解析未在时限内返回(疑似同步死循环)")
	}
}

func TestReadStructAtTruncatedReturns(t *testing.T) {
	done := make(chan *tarsReader, 1)
	go func() {
		reader := &tarsReader{buf: []byte{beginByte(4), 0x26, 0x03, 'a', 'b', 'c'}}
		done <- reader.readStructAt(4)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("readStructAt 截断时未返回(疑似同步死循环)")
	}
}

func TestDecodeZeroTagFrame(t *testing.T) {
	sender := &tarsWriter{}
	sender.writeString("ok", 3)
	textField := &tarsWriter{}
	textField.writeString("hello-after-zero", 3)
	message := []byte{beginByte(0), zeroByte(2)}
	message = append(message, sender.bytes()...)
	message = append(message, endByte(0))
	message = append(message, textField.bytes()...)
	items := decodeDanmakuFrame(hyFrame(message))
	if len(items) != 1 || items[0].Text != "hello-after-zero" {
		t.Fatalf("items = %+v(含 ZERO_TAG 的正常帧被提前截断)", items)
	}
}

func TestDecodeColorFrame(t *testing.T) {
	sender := &tarsWriter{}
	sender.writeString("abc", 2)
	textField := &tarsWriter{}
	textField.writeString("colorful", 3)
	bullet := &tarsWriter{}
	bullet.writeInt(0xff0000, 0)
	message := []byte{beginByte(0)}
	message = append(message, sender.bytes()...)
	message = append(message, endByte(0))
	message = append(message, textField.bytes()...)
	message = append(message, beginByte(6))
	message = append(message, bullet.bytes()...)
	message = append(message, endByte(6))
	items := decodeDanmakuFrame(hyFrame(message))
	if len(items) != 1 || items[0].Color == nil || *items[0].Color != "#ff0000" {
		t.Fatalf("items = %+v", items)
	}
}

func TestWriterIntWidths(t *testing.T) {
	cases := []struct {
		n    int64
		tag  int
		want []byte
	}{
		{0, 5, []byte{0x5c}},
		{127, 0, []byte{0x00, 0x7f}},
		{-1, 0, []byte{0x00, 0xff}},
		{128, 0, []byte{0x01, 0x00, 0x80}},
		{-129, 0, []byte{0x01, 0xff, 0x7f}},
		{32768, 0, []byte{0x02, 0x00, 0x00, 0x80, 0x00}},
		{2147483648, 0, []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00}},
		{1, 15, []byte{0xf0, 0x0f, 0x01}},
	}
	for _, tc := range cases {
		writer := &tarsWriter{}
		writer.writeInt(tc.n, tc.tag)
		if !bytes.Equal(writer.bytes(), tc.want) {
			t.Errorf("writeInt(%d,%d) = % x, want % x", tc.n, tc.tag, writer.bytes(), tc.want)
		}
	}
}

func TestWriterStringAndBytes(t *testing.T) {
	writer := &tarsWriter{}
	writer.writeString("hi", 2)
	if want := []byte{0x26, 0x02, 'h', 'i'}; !bytes.Equal(writer.bytes(), want) {
		t.Errorf("writeString = % x", writer.bytes())
	}

	long := strings.Repeat("a", 256)
	writer = &tarsWriter{}
	writer.writeString(long, 0)
	want := append([]byte{0x07, 0x00, 0x00, 0x01, 0x00}, []byte(long)...)
	if !bytes.Equal(writer.bytes(), want) {
		t.Errorf("writeString(256) 头 = % x", writer.bytes()[:5])
	}

	writer = &tarsWriter{}
	writer.writeBytes([]byte{0xaa}, 0)
	if want := []byte{0x0d, 0x00, 0x00, 0x01, 0xaa}; !bytes.Equal(writer.bytes(), want) {
		t.Errorf("writeBytes = % x", writer.bytes())
	}
}

func TestReaderRoundTrip(t *testing.T) {
	writer := &tarsWriter{}
	writer.writeInt(12345, 3)
	writer.writeBool(true, 4)
	writer.writeString("虎牙", 5)
	writer.writeBytes([]byte{1, 2, 3}, 7)
	reader := &tarsReader{buf: writer.bytes()}
	if got := reader.readInt(3); got != 12345 {
		t.Errorf("readInt = %d", got)
	}
	if !reader.readBool(4) {
		t.Error("readBool = false")
	}
	if got := reader.readString(5); got != "虎牙" {
		t.Errorf("readString = %q", got)
	}
	if got := reader.readBytes(7); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Errorf("readBytes = %v", got)
	}
	if got := reader.readInt(9); got != 0 {
		t.Errorf("越界 tag = %d", got)
	}
}

func TestRGBToHex(t *testing.T) {
	if got := rgbToHex(0xff0000); got != "#ff0000" {
		t.Errorf("rgbToHex = %q", got)
	}
	if got := rgbToHex(0x00ff00); got != "#00ff00" {
		t.Errorf("rgbToHex = %q", got)
	}
}
