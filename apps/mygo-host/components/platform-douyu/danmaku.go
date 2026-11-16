// douyu 弹幕:wss://danmuproxy.douyu.com:8506 + STT 纯文本(无鉴权)。
//
// 协议(12B 小端头 + STT body,帧长 = fullLen + 4):
//   fullLen/fullLen2 + packType(689 上行 / 690 下行) + encrypted + reserved,
//   body = fullLen-9 字节,末尾 0x00;body 为 `k@=v/k@=v/`(转义 @S→/、@A→@)。
// ⚠️ 一个 WebSocket 帧可能粘多个 STT 包,必须逐包全解(pure_live 同款)。

package douyu

import (
	"context"
	"encoding/binary"
	"math"
	"strings"
	"time"

	"mygo-host/platform"
)

const (
	wsURL             = "wss://danmuproxy.douyu.com:8506"
	heartbeatInterval = 45 * time.Second
)

// douyuColors 斗鱼专属 6 色索引(1红 2蓝 3绿 4橙 5紫 6粉),非真 hex。
var douyuColors = map[int]string{
	1: "#ff0000",
	2: "#1e87f0",
	3: "#7ac84b",
	4: "#ff7f00",
	5: "#9b39f4",
	6: "#ff69b4",
}

// douyuFrame 12B 小端头 + STT body + 末尾 0x00 → 一帧(上行 packType=689)。
func douyuFrame(body string) []byte {
	bodyBytes := []byte(body)
	full := 9 + len(bodyBytes)
	frame := make([]byte, 12+len(bodyBytes)+1)
	binary.LittleEndian.PutUint32(frame[0:], uint32(full))
	binary.LittleEndian.PutUint32(frame[4:], uint32(full))
	binary.LittleEndian.PutUint16(frame[8:], 689)
	copy(frame[12:], bodyBytes)
	return frame
}

// sttParse `k@=v/` 切分 + @S/@A 转义(与 dart sttToJObject 单层等价)。
func sttParse(str string) map[string]string {
	obj := make(map[string]string)
	for _, field := range strings.Split(str, "/") {
		if field == "" {
			continue
		}
		i := strings.Index(field, "@=")
		if i < 0 {
			continue
		}
		value := strings.ReplaceAll(field[i+2:], "@S", "/")
		obj[field[:i]] = strings.ReplaceAll(value, "@A", "@")
	}
	return obj
}

// parseDouyuFrame 解析一个 WS 帧(可能粘多包)→ 弹幕。
func parseDouyuFrame(data []byte) []platform.DanmakuItem {
	items := make([]platform.DanmakuItem, 0)
	off := 0
	for off+12 <= len(data) {
		full := int(binary.LittleEndian.Uint32(data[off:]))
		frameLen := full + 4
		if full < 9 || off+frameLen > len(data) {
			break
		}
		if binary.LittleEndian.Uint16(data[off+8:]) == 690 {
			bodyLen := full - 9
			obj := sttParse(string(data[off+12 : off+12+bodyLen]))
			// dms 缺失 = 屏蔽弹幕(禁言列表),与 TS 同款跳过。
			if _, present := obj["dms"]; obj["type"] == "chatmsg" && present {
				items = append(items, platform.DanmakuItem{
					Text:  obj["txt"],
					User:  platform.Str(obj["nn"]),
					Color: douyuColorOf(obj["col"]),
				})
			}
		}
		off += frameLen
	}
	return items
}

// douyuColorOf 等价 DOUYU_COLORS[Number(col) || 0];未命中省略 color。
func douyuColorOf(col string) *string {
	n, ok := platform.JSNumber(col)
	if !ok {
		n = 0
	}
	if n == 0 || math.Trunc(n) != n {
		return nil
	}
	hex, found := douyuColors[int(n)]
	if !found {
		return nil
	}
	return platform.Str(hex)
}

// danmakuStream 订阅即建连(进房 + 心跳),退订断开;生命周期走共享
// platform.CreateWSStream(重连/心跳/熔断),平台差异只剩协议帧。
func danmakuStream(sc *platform.ServiceContext, roomID string) platform.DanmakuStream {
	return platform.CreateWSStream(context.Background(), platform.WSStreamOptions{
		URL: func() string { return wsURL },
		OnOpen: func(conn *platform.WSConn) {
			_ = conn.Write(douyuFrame("type@=loginreq/roomid@" + roomID + "/"))
			_ = conn.Write(douyuFrame("type@=joingroup/rid@" + roomID + "/gid@=-9999/"))
		},
		OnMessage: func(data []byte, _ *platform.WSConn) ([]platform.DanmakuItem, error) {
			return parseDouyuFrame(data), nil
		},
		Heartbeat:   func() []byte { return douyuFrame("type@=mrkl/") },
		HeartbeatMs: int(heartbeatInterval / time.Millisecond),
	}, func(format string, args ...any) { sc.Log("douyu: 弹幕"+format, args...) })
}
