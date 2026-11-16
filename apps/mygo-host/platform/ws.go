// WS 客户端封装(appHost.ws 的 Go 等价):gorilla/websocket 直连,
// 自定义握手 header(huya/bili-live/douyu 弹幕握手需要)。
// 消费方(各平台弹幕实现)自持读循环 goroutine 与心跳。

package platform

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WSConn 平台侧的 WebSocket 连接(并发安全:Write 可多 goroutine;Read 单 goroutine)。
type WSConn struct {
	conn *websocket.Conn
}

// Dial 建连(带自定义 header;默认 15s 握手超时)。
func Dial(ctx context.Context, url string, headers map[string]string) (*WSConn, error) {
	dialer := &websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	httpHeader := http.Header{}
	for k, v := range headers {
		httpHeader.Set(k, v)
	}
	conn, _, err := dialer.DialContext(ctx, url, httpHeader)
	if err != nil {
		return nil, err
	}
	return &WSConn{conn: conn}, nil
}

// Write 发二进制帧。
func (c *WSConn) Write(data []byte) error {
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// WriteText 发文本帧(douyu STT 是文本协议)。
func (c *WSConn) WriteText(data string) error {
	return c.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

// Read 阻塞读下一帧;返回是否文本帧与 payload。
func (c *WSConn) Read() (text bool, data []byte, err error) {
	msgType, data, err := c.conn.ReadMessage()
	return msgType == websocket.TextMessage, data, err
}

// Close 关闭连接(幂等)。
func (c *WSConn) Close() error { return c.conn.Close() }
