// 虎牙弹幕:进房参数取自 m.huya.com 房间页 HTML(HNF_GLOBAL_INIT 常把 lYyid 置 0,
// 用正则取真实值);wss://cdnws.api.huya.com + Tars 进房帧;60s 固定心跳;
// 断线重连/熔断由共享 platform.CreateWSStream 统一(对齐 TS 组合语义)。

package huya

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"mygo-host/platform"
)

const (
	danmakuWSURL = "wss://cdnws.api.huya.com"
	heartbeatMs  = 60000
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

// danmakuStream 订阅即异步 setup(房间页提进房参数)→ 建 WS;生命周期走共享
// platform.DeferredStream + CreateWSStream(退订零泄漏/重连/心跳/熔断)。
func danmakuStream(sc *platform.ServiceContext, roomID string) platform.DanmakuStream {
	logf := func(format string, args ...any) { sc.Log("huya danmaku "+format, args...) }
	return platform.DeferredStream(
		func() (danmakuArgs, error) {
			return fetchDanmakuArgs(context.Background(), sc, roomID)
		},
		func(args danmakuArgs, onItems func([]platform.DanmakuItem)) func() {
			return platform.CreateWSStream(context.Background(), platform.WSStreamOptions{
				URL:    func() string { return danmakuWSURL },
				OnOpen: func(conn *platform.WSConn) { _ = conn.Write(encodeJoin(args)) },
				OnMessage: func(data []byte, _ *platform.WSConn) ([]platform.DanmakuItem, error) {
					return decodeDanmakuFrame(data), nil
				},
				Heartbeat:   func() []byte { return heartbeatFrame },
				HeartbeatMs: heartbeatMs,
			}, logf)(onItems)
		},
		func(err error) { logf("初始化失败: %v", err) },
	)
}
