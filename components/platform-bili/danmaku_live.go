// bilibili 直播弹幕：getDanmuInfo（wbi 签名）→ wss → op=7 认证 → 心跳 → op=5 弹幕。
// 协议（16B 大端头 + JSON body）：op 2 心跳 / 3 心跳回 / 5 通知 / 7 认证 / 8 进房回；
// protover 0 JSON / 2 zlib（请求 protover=2）。op=8 code!=0 → 认证失败，走 refresh 重连。

package bili

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"mygo-host/platform"
)

const (
	heartbeatMs          = 60000
	maxCredentialRefresh = 3
)

// biliFrame 16B 大端头 + JSON body。
func biliFrame(op int, bodyObj any, protover uint16) []byte {
	body, _ := json.Marshal(bodyObj)
	frame := make([]byte, 16+len(body))
	binary.BigEndian.PutUint32(frame[0:], uint32(16+len(body)))
	binary.BigEndian.PutUint16(frame[4:], 16)
	binary.BigEndian.PutUint16(frame[6:], protover)
	binary.BigEndian.PutUint32(frame[8:], uint32(op))
	binary.BigEndian.PutUint32(frame[12:], 1)
	copy(frame[16:], body)
	return frame
}

// biliPacket 一个解包后的协议帧。
type biliPacket struct {
	op   uint32
	ver  uint16
	body []byte
}

// parseBiliPackets 逐包切分 16B 头粘包。
func parseBiliPackets(buf []byte) []biliPacket {
	var out []biliPacket
	off := 0
	for off+16 <= len(buf) {
		total := int(binary.BigEndian.Uint32(buf[off:]))
		headerLen := int(binary.BigEndian.Uint16(buf[off+4:]))
		ver := binary.BigEndian.Uint16(buf[off+6:])
		op := binary.BigEndian.Uint32(buf[off+8:])
		end := off + total
		if end > len(buf) {
			end = len(buf)
		}
		if headerLen >= 16 && end > off+headerLen {
			out = append(out, biliPacket{op: op, ver: ver, body: buf[off+headerLen : end]})
		}
		if total <= 0 {
			break
		}
		off += total
	}
	return out
}

// inflateBiliBody op=5 消息体解压：ver 2 = zlib。ver 3（brotli）不解压原样返回
// （请求的就是 protover=2，服务端不会回 brotli；与 TS 解压失败兜底同语义）。
func inflateBiliBody(ver uint16, data []byte) []byte {
	if ver != 2 {
		return data
	}
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return data
	}
	defer r.Close()
	plain, err := io.ReadAll(r)
	if err != nil {
		return data
	}
	return plain
}

// isAuthRejected op=8 进房回 code!=0（空 body / 非 JSON 视为成功）。
func isAuthRejected(p biliPacket) bool {
	if p.op != 8 {
		return false
	}
	text := strings.TrimSpace(string(p.body))
	if text == "" {
		return false
	}
	var msg struct {
		Code *float64 `json:"code"`
	}
	if err := json.Unmarshal([]byte(text), &msg); err != nil {
		return false
	}
	return jnumPtr(msg.Code) != 0
}

// parseBiliDanmakuFrame 解一帧 WS 数据（op=5 弹幕 / op=8 鉴权结果）。
func parseBiliDanmakuFrame(data []byte) ([]platform.DanmakuItem, error) {
	items := []platform.DanmakuItem{}
	for _, p := range parseBiliPackets(data) {
		if isAuthRejected(p) {
			return nil, fmt.Errorf("bili:live 弹幕鉴权被拒(op=8)")
		}
		if p.op != 5 {
			continue
		}
		text := string(inflateBiliBody(p.ver, p.body))
		for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r <= 0x1f }) {
			var msg struct {
				Cmd  string `json:"cmd"`
				Info []any  `json:"info"`
			}
			if json.Unmarshal([]byte(line), &msg) != nil || msg.Cmd != "DANMU_MSG" {
				continue
			}
			text, user, color := danmuFields(msg.Info)
			if text == "" {
				continue
			}
			items = append(items, platform.DanmakuItem{
				Text:  text,
				User:  &user,
				Color: platform.Str(argbToHex(color)),
			})
		}
	}
	return items, nil
}

// danmuFields DANMU_MSG.info → 文本 / 昵称 / ARGB 色值。
func danmuFields(info []any) (string, string, int) {
	text := ""
	if len(info) > 1 {
		text = platform.JSString(info[1])
	}
	user := ""
	if len(info) > 2 {
		if arr := jlist(info[2]); len(arr) > 1 {
			user = platform.JSString(arr[1])
		}
	}
	color := 0xffffff
	if len(info) > 0 {
		if arr := jlist(info[0]); len(arr) > 3 {
			color = jint(arr[3])
		}
	}
	return text, user, color
}

// danmuCred 一次 getDanmuInfo 的结果（token 有寿命，重连前须刷新）。
type danmuCred struct {
	host    string
	wssPort int
	token   string
	uid     int
	buvid3  string
}

type liveCreds struct {
	mu  sync.Mutex
	cur danmuCred
}

func (l *liveCreds) get() danmuCred {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cur
}

func (l *liveCreds) set(c danmuCred) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cur = c
}

func (c *client) getDanmuInfo(ctx context.Context, roomID string, cookie *string) (danmuCred, error) {
	// 认证帧身份只用调用方显式 cookie（TS 同：未传/空串 = 匿名 uid=0 +
	// 匿名指纹 buvid3，不掺 provider 登录态）；HTTP 链由 signWeb/getJSON
	// 内部各自三档解析，登录 cookie 可用。
	uid := 0
	if cookie != nil && *cookie != "" {
		explicit := *cookie
		if v, err := c.navMid(ctx, &explicit); err == nil {
			uid = v
		}
	}
	buvid3 := extractCookie(deref(cookie), "buvid3")
	if buvid3 == "" {
		if v, err := c.anonBuvid3(ctx); err == nil {
			buvid3 = v
		}
	}
	signed, err := c.signWeb(ctx, "id="+roomID, cookie)
	if err != nil {
		return danmuCred{}, err
	}
	url := "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?" + signed
	raw, _, err := c.getJSON(ctx, url, reqOptions{Referer: liveBase + "/", Cookie: cookie})
	if err != nil {
		return danmuCred{}, err
	}
	var resp struct {
		Data *struct {
			Token    *string `json:"token"`
			HostList []struct {
				Host    *string  `json:"host"`
				WssPort *float64 `json:"wss_port"`
			} `json:"host_list"`
		} `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return danmuCred{}, err
	}
	host, token, wssPort := "", "", 443
	if resp.Data != nil && len(resp.Data.HostList) > 0 {
		host = deref(resp.Data.HostList[0].Host)
		if resp.Data.HostList[0].WssPort != nil {
			wssPort = int(*resp.Data.HostList[0].WssPort)
		}
		token = deref(resp.Data.Token)
	}
	if host == "" || token == "" {
		return danmuCred{}, fmt.Errorf("bili:live danmaku: no host/token for room %s", roomID)
	}
	return danmuCred{host: host, wssPort: wssPort, token: token, uid: uid, buvid3: buvid3}, nil
}

// liveDanmakuStream 订阅时 getDanmuInfo → 建 WS（认证 → 心跳 → 收弹幕），退订断开。
func (c *client) liveDanmakuStream(roomID string, cookie *string) platform.DanmakuStream {
	return platform.DeferredStream(
		func() (danmuCred, error) {
			return c.getDanmuInfo(context.Background(), roomID, cookie)
		},
		func(initial danmuCred, onItems func([]platform.DanmakuItem)) func() {
			creds := &liveCreds{cur: initial}
			refreshes := 0
			refresh := func() (bool, error) {
				if refreshes >= maxCredentialRefresh {
					return false, nil
				}
				refreshes++
				next, err := c.getDanmuInfo(context.Background(), roomID, cookie)
				if err != nil {
					return false, err
				}
				creds.set(next)
				return true, nil
			}
			return platform.CreateWSStream(context.Background(), platform.WSStreamOptions{
				URL: func() string {
					cur := creds.get()
					return fmt.Sprintf("wss://%s:%d/sub", cur.host, cur.wssPort)
				},
				Refresh: refresh,
				OnOpen: func(conn *platform.WSConn) {
					cur := creds.get()
					roomNum, _ := strconv.Atoi(roomID)
					payload := map[string]any{
						"uid": cur.uid, "roomid": roomNum, "protover": 2,
						"buvid": cur.buvid3, "platform": "web", "type": 2, "key": cur.token,
					}
					_ = conn.Write(biliFrame(7, payload, 0))
				},
				Heartbeat:   func() []byte { return biliFrame(2, map[string]any{}, 0) },
				HeartbeatMs: heartbeatMs,
				OnMessage: func(data []byte, _ *platform.WSConn) ([]platform.DanmakuItem, error) {
					return parseBiliDanmakuFrame(data)
				},
				OnClose: func(code int, reason string) {
					_ = code
					_ = reason
				},
			}, func(format string, args ...any) { c.log("bili: "+format, args...) })(onItems)
		},
		func(err error) { c.log("bili-live: 弹幕初始化失败: %v", err) },
	)
}
