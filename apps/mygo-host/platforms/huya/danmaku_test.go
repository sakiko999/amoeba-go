// 弹幕离线测试:进房参数提取(HTML 正则与兜底)、进房帧字节黄金集、
// 心跳固定字节。

package huya

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseDanmakuArgs(t *testing.T) {
	html := `<script>var x = {"lYyid":12345,"lChannelId":678,"lSubChannelId":90};</script>`
	args, err := parseDanmakuArgs(html, "1")
	if err != nil {
		t.Fatal(err)
	}
	if args != (danmakuArgs{ayyuid: 12345, topSid: 678, subSid: 90}) {
		t.Errorf("args = %+v", args)
	}

	args, err = parseDanmakuArgs(`{"lYyid":777}`, "2")
	if err != nil || args.topSid != 777 || args.subSid != 777 {
		t.Errorf("top/sub 应回退 ayyuid: %+v, err = %v", args, err)
	}

	if _, err := parseDanmakuArgs(`{}`, "3"); err == nil || !strings.Contains(err.Error(), "无 lYyid") {
		t.Errorf("缺 lYyid 应报错: %v", err)
	}
}

func TestEncodeJoinBytes(t *testing.T) {
	got := encodeJoin(danmakuArgs{ayyuid: 123, topSid: 456, subSid: 456})
	want := []byte{
		0x00, 0x01, // wscmd tag0=1
		0x1d, 0x00, 0x00, 0x10, // tag1=SIMPLE_LIST, 内层 BYTE 头, 长度 16
		0x00, 0x7b, // join tag0=123(BYTE)
		0x10, 0x01, // join tag1=true
		0x26, 0x00, // join tag2=""
		0x36, 0x00, // join tag3=""
		0x41, 0x01, 0xc8, // join tag4=456(SHORT)
		0x51, 0x01, 0xc8, // join tag5=456(SHORT)
		0x6c, // join tag6=0(ZERO_TAG)
		0x7c, // join tag7=0(ZERO_TAG)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("join 帧 = % x\nwant      % x", got, want)
	}
}

func TestHeartbeatFrame(t *testing.T) {
	want := []byte{0x00, 0x14, 0x1d, 0x00, 0x0c, 0x2c, 0x36, 0x00}
	if !bytes.Equal(heartbeatFrame, want) {
		t.Errorf("heartbeat = % x", heartbeatFrame)
	}
}
