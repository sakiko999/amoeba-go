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
	"math/rand"
	"strings"
	"sync"
	"time"

	"mygo-host/platform"
)

const (
	wsURL                = "wss://danmuproxy.douyu.com:8506"
	heartbeatInterval    = 45 * time.Second
	maxReconnectAttempts = 10
	maxReconnectDelay    = 30 * time.Second
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
	n, ok := jsNumber(col)
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

// danmakuStream 订阅即建连(进房 + 心跳),退订断开。
func danmakuStream(sc *platform.ServiceContext, roomID string) platform.DanmakuStream {
	return func(onItems func([]platform.DanmakuItem)) func() {
		session := &wsSession{
			sc:      sc,
			roomID:  roomID,
			onItems: onItems,
			done:    make(chan struct{}),
		}
		go session.connect()
		return session.stop
	}
}

// wsSession 单次订阅的 WS 生命周期:读循环 + 心跳 + 指数退避重连。
type wsSession struct {
	sc      *platform.ServiceContext
	roomID  string
	onItems func([]platform.DanmakuItem)
	done    chan struct{}

	mu             sync.Mutex
	stopped        bool
	conn           *platform.WSConn
	reconnectTimer *time.Timer
	attempts       int
}

func (s *wsSession) connect() {
	conn, err := platform.Dial(context.Background(), wsURL, nil)
	if err != nil {
		s.log("弹幕握手失败: %v", err)
		s.scheduleReconnect()
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	s.conn = conn
	s.attempts = 0
	s.mu.Unlock()

	s.run(conn)
	_ = conn.Close()
	s.mu.Lock()
	if s.conn == conn {
		s.conn = nil
	}
	stopped := s.stopped
	s.mu.Unlock()
	if !stopped {
		s.scheduleReconnect()
	}
}

func (s *wsSession) run(conn *platform.WSConn) {
	s.write(conn, douyuFrame("type@=loginreq/roomid@"+s.roomID+"/"))
	s.write(conn, douyuFrame("type@=joingroup/rid@"+s.roomID+"/gid@=-9999/"))
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go s.heartbeat(conn, heartbeatDone)
	for {
		_, data, err := conn.Read()
		if err != nil {
			return
		}
		if items := parseDouyuFrame(data); len(items) > 0 {
			s.deliver(items)
		}
	}
}

func (s *wsSession) heartbeat(conn *platform.WSConn, done <-chan struct{}) {
	s.write(conn, douyuFrame("type@=mrkl/"))
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-s.done:
			return
		case <-ticker.C:
			s.write(conn, douyuFrame("type@=mrkl/"))
		}
	}
}

func (s *wsSession) write(conn *platform.WSConn, frame []byte) {
	if s.isStopped() {
		return
	}
	_ = conn.Write(frame)
}

func (s *wsSession) deliver(items []platform.DanmakuItem) {
	if s.isStopped() {
		return
	}
	s.onItems(items)
}

func (s *wsSession) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

// scheduleReconnect 指数退避 1s→2s→4s…(上限 30s)+ ±15% full jitter。
func (s *wsSession) scheduleReconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	if s.attempts >= maxReconnectAttempts {
		s.log("弹幕重连 %d 次后放弃", s.attempts)
		return
	}
	base := float64(time.Second) * math.Pow(2, float64(s.attempts))
	if base > float64(maxReconnectDelay) {
		base = float64(maxReconnectDelay)
	}
	delay := time.Duration(base * (0.85 + rand.Float64()*0.3))
	s.attempts++
	s.log("弹幕重连第 %d 次,%v 后", s.attempts, delay)
	s.reconnectTimer = time.AfterFunc(delay, s.connect)
}

// stop 退订:断开 WS + 清心跳/重连定时器(幂等)。
func (s *wsSession) stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.done)
	timer := s.reconnectTimer
	conn := s.conn
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (s *wsSession) log(format string, args ...any) {
	if s.sc != nil {
		s.sc.Log("douyu: "+format, args...)
	}
}
