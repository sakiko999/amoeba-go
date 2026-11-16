// 虎牙弹幕:进房参数取自 m.huya.com 房间页 HTML(HNF_GLOBAL_INIT 常把 lYyid 置 0,
// 用正则取真实值);wss://cdnws.api.huya.com + Tars 进房帧;60s 固定心跳;
// 断线按指数退避重连(±15% 抖动,连续 10 次放弃,成功连接清零计数)。
// 组合语义对齐 TS deferredStream + createWsStream:订阅即异步 setup + 建连,
// 退订断开且不再建流(连接零泄漏)。

package huya

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"time"

	"mygo-host/platform"
)

const (
	danmakuWSURL   = "wss://cdnws.api.huya.com"
	heartbeatMs    = 60000
	maxAttempts    = 10
	maxReconnectMs = 30000
)

var (
	// heartbeatFrame 心跳固定字节(dart base64.decode("ABQdAAwsNgBM"))。
	heartbeatFrame  = []byte{0x00, 0x14, 0x1d, 0x00, 0x0c, 0x2c, 0x36, 0x00}
	lYyidRe         = regexp.MustCompile(`lYyid":([0-9]+)`)
	lChannelIDRe    = regexp.MustCompile(`lChannelId":([0-9]+)`)
	lSubChannelIDRe = regexp.MustCompile(`lSubChannelId":([0-9]+)`)
)

// danmakuArgs 弹幕进房参数。
type danmakuArgs struct {
	ayyuid int64
	topSid int64
	subSid int64
}

// parseDanmakuArgs 从房间页提取进房参数;top/sub 缺失回退 ayyuid。
func parseDanmakuArgs(html, roomID string) (danmakuArgs, error) {
	args := danmakuArgs{
		ayyuid: matchNumber(html, lYyidRe),
		topSid: matchNumber(html, lChannelIDRe),
		subSid: matchNumber(html, lSubChannelIDRe),
	}
	if args.ayyuid == 0 {
		return danmakuArgs{}, fmt.Errorf("huya danmaku: 无 lYyid(房间 %s 异常)", roomID)
	}
	if args.topSid == 0 {
		args.topSid = args.ayyuid
	}
	if args.subSid == 0 {
		args.subSid = args.ayyuid
	}
	return args, nil
}

func matchNumber(html string, re *regexp.Regexp) int64 {
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// fetchDanmakuArgs 取房间页(移动 UA)。
func fetchDanmakuArgs(ctx context.Context, sc *platform.ServiceContext, roomID string) (danmakuArgs, error) {
	html, err := sc.HTTP.GetText(ctx, mHuya+"/"+roomID, map[string]string{"user-agent": uaMobile})
	if err != nil {
		return danmakuArgs{}, err
	}
	return parseDanmakuArgs(html, roomID)
}

// encodeJoin 进房帧(wscmd{tag0=1, tag1=进房数据};与 dart getJoinData 同构)。
func encodeJoin(args danmakuArgs) []byte {
	join := &tarsWriter{}
	join.writeInt(args.ayyuid, 0)
	join.writeBool(true, 1)
	join.writeString("", 2)
	join.writeString("", 3)
	join.writeInt(args.topSid, 4) // dart: tid/sid 都传 topSid
	join.writeInt(args.topSid, 5)
	join.writeInt(0, 6)
	join.writeInt(0, 7)
	command := &tarsWriter{}
	command.writeInt(1, 0)
	command.writeBytes(join.bytes(), 1)
	return command.bytes()
}

// danmakuStream 订阅即开始,返回退订函数。
func danmakuStream(sc *platform.ServiceContext, roomID string) platform.DanmakuStream {
	return func(onItems func([]platform.DanmakuItem)) func() {
		session := &danmakuSession{
			sc:      sc,
			roomID:  roomID,
			onItems: onItems,
			stopCh:  make(chan struct{}),
		}
		go session.run(context.Background())
		return session.stop
	}
}

type danmakuSession struct {
	sc       *platform.ServiceContext
	roomID   string
	onItems  func([]platform.DanmakuItem)
	stopCh   chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	conn     *platform.WSConn
}

func (s *danmakuSession) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()
		if conn != nil {
			_ = conn.Close()
		}
	})
}

func (s *danmakuSession) stopped() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

func (s *danmakuSession) setConn(conn *platform.WSConn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
}

// run 异步 setup → 连接循环;返回即流结束(退订/初始化失败/重连熔断)。
func (s *danmakuSession) run(ctx context.Context) {
	args, err := fetchDanmakuArgs(ctx, s.sc, s.roomID)
	if err != nil {
		if !s.stopped() {
			s.sc.Log("huya danmaku 初始化失败: %v", err)
		}
		return
	}
	attempts := 0
	for !s.stopped() {
		conn, err := platform.Dial(ctx, danmakuWSURL, nil)
		if err != nil {
			s.sc.Log("huya danmaku 连接失败: %v", err)
			if !s.waitReconnect(attempts) {
				return
			}
			attempts++
			continue
		}
		s.setConn(conn)
		if s.stopped() {
			_ = conn.Close()
			return
		}
		attempts = 0
		_ = conn.Write(encodeJoin(args))
		heartbeatStop := make(chan struct{})
		go s.heartbeat(conn, heartbeatStop)
		s.readLoop(conn)
		close(heartbeatStop)
		_ = conn.Close()
		s.setConn(nil)
		if s.stopped() || !s.waitReconnect(attempts) {
			return
		}
		attempts++
	}
}

func (s *danmakuSession) readLoop(conn *platform.WSConn) {
	for {
		_, data, err := conn.Read()
		if err != nil || s.stopped() {
			return
		}
		if items := decodeDanmakuFrame(data); len(items) > 0 {
			s.onItems(items)
		}
	}
}

// heartbeat 连上即发一帧,之后每 60s 一帧(TS onReady 的 hb() + setInterval)。
func (s *danmakuSession) heartbeat(conn *platform.WSConn, stop <-chan struct{}) {
	send := func() {
		if !s.stopped() {
			_ = conn.Write(heartbeatFrame)
		}
	}
	send()
	ticker := time.NewTicker(heartbeatMs * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			send()
		}
	}
}

// waitReconnect TS scheduleReconnect:attempts 达上限放弃,否则按
// min(1000*2^attempts, 30s) × [0.85,1.15] 抖动等待;false = 放弃。
func (s *danmakuSession) waitReconnect(attempts int) bool {
	if attempts >= maxAttempts {
		s.sc.Log("huya danmaku 重连 %d 次后放弃", attempts)
		return false
	}
	base := 1000 * (1 << attempts)
	if base > maxReconnectMs {
		base = maxReconnectMs
	}
	delay := time.Duration(math.Round(float64(base)*(0.85+rand.Float64()*0.3))) * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-s.stopCh:
		return false
	case <-timer.C:
		return true
	}
}
