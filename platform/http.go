// appHost.http 的 Go 等价:统一 HTTP 传输面。所有平台请求都走这里(不发直连),
// 未来治理(速率/风控)挂在这一层。响应 body 恒为 text;set-cookie 以 "\n" 连接
// 回传(bili 扫码登录/douyin ttwid warmup 依赖它)。

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUA 兜底 UA:TS 侧 electron net.fetch 不设 UA 时发 Chromium 默认 UA,
// Go http 默认无 UA(部分站点拒绝),平台未显式指定时补这个。
var DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// Request 统一请求(对齐 TS HttpRequest)。
type Request struct {
	URL     string
	Method  string // 缺省 GET
	Headers map[string]string
	Body    string
	// Timeout 单请求超时;0 = 不限。
	Timeout time.Duration
}

// Response 统一响应:status + headers(set-cookie 多值合并 "\n")+ bodyText。
type Response struct {
	Status  int
	Headers map[string]string
	Body    string
}

// HostClient 平台统一 HTTP 客户端(并发安全;宿主内单例)。
type HostClient struct {
	HTTP *http.Client
}

// NewHostClient 建客户端。
func NewHostClient() *HostClient {
	return &HostClient{HTTP: &http.Client{}}
}

// Do 发请求。非 2xx **不**报错(调用方决定诊断/抛出,与 TS httpGet 一致)。
func (c *HostClient) Do(ctx context.Context, req Request) (*Response, error) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return nil, err
	}
	hasUA := false
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
		if strings.EqualFold(k, "user-agent") {
			hasUA = true
		}
	}
	if !hasUA {
		httpReq.Header.Set("User-Agent", DefaultUA)
	}
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
	}
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	text, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if raw := res.Header.Values("Set-Cookie"); len(raw) > 0 {
		headers["set-cookie"] = strings.Join(raw, "\n")
	}
	return &Response{Status: res.StatusCode, Headers: headers, Body: string(text)}, nil
}

// GetText 便捷 GET:text 响应,非 2xx 报错。
func (c *HostClient) GetText(ctx context.Context, url string, headers map[string]string) (string, error) {
	res, err := c.Do(ctx, Request{URL: url, Headers: headers})
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", res.Status, url)
	}
	return res.Body, nil
}

// GetJSON 便捷 GET:json 响应;空 body 返回 nil(「无数据」语义,与 TS httpJson 一致)。
func (c *HostClient) GetJSON(ctx context.Context, url string, headers map[string]string) (RawJSON, error) {
	res, err := c.Do(ctx, Request{URL: url, Headers: headers})
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.Status, url)
	}
	if strings.TrimSpace(res.Body) == "" {
		return nil, nil
	}
	return RawJSON(res.Body), nil
}

// UnmarshalJSON 把 GetJSON 的结果解码进 out;nil(空 body)时 out 保持零值。
func UnmarshalJSON(raw RawJSON, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
