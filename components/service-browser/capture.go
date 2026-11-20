// CaptureNavigate 的被动捕获:Network.enable 后只收集页面自己发出的匹配
// 文本响应,不主动发请求(接口重放会被风控)。事件订阅器只做内存记录、
// 不阻塞读循环;getResponseBody 由收集循环同步发出,因此返回前在途 body
// 一定已取完。超时返回已收集部分,不报错。

package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"mygo-host/platform"
)

// 与 TS capturePage 原文一致:只取文本类响应。
var textMIME = regexp.MustCompile(`(?i)^(text/|application/(json|xml|javascript|x-www-form-urlencoded)|application/[^;]+\+(json|xml))`)

type finishedReq struct {
	requestID string
	url       string
}

type captureState struct {
	sessionID string
	match     func(*url.URL) bool

	mu           sync.Mutex
	inflight     map[string]*url.URL
	mime         map[string]string
	finished     []finishedReq
	lastActivity time.Time
}

func newCaptureState(sessionID string, match func(*url.URL) bool) *captureState {
	return &captureState{
		sessionID:    sessionID,
		match:        match,
		inflight:     map[string]*url.URL{},
		mime:         map[string]string{},
		lastActivity: time.Now(),
	}
}

func (p *page) CaptureNavigate(ctx context.Context, rawURL string, match func(*url.URL) bool, idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	if match == nil {
		return nil, errors.New("browser: CaptureNavigate 缺少 match")
	}
	if err := p.checkURL(rawURL); err != nil {
		return nil, err
	}
	before, _ := p.Evaluate(ctx, "performance.timeOrigin")
	if err := p.enableNetwork(ctx); err != nil {
		return nil, err
	}

	st := newCaptureState(p.sessionID, match)
	unsubscribe := p.conn.subscribe(st.handle)
	defer unsubscribe()

	loaderID, err := p.navigate(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return p.collectCaptured(ctx, st, before, loaderID == "", idle, timeout)
}

// Capture 在现有文档上执行 trigger(如点击切 Tab)并收集其后的匹配响应。
// 不导航;sameDoc=true 让静默窗口只要求 readyState complete。
func (p *page) Capture(ctx context.Context, trigger func() error, match func(*url.URL) bool, idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	if match == nil {
		return nil, errors.New("browser: Capture 缺少 match")
	}
	if trigger == nil {
		return nil, errors.New("browser: Capture 缺少 trigger")
	}
	if err := p.enableNetwork(ctx); err != nil {
		return nil, err
	}

	st := newCaptureState(p.sessionID, match)
	unsubscribe := p.conn.subscribe(st.handle)
	defer unsubscribe()

	if err := trigger(); err != nil {
		return nil, err
	}
	return p.collectCaptured(ctx, st, nil, true, idle, timeout)
}

func (p *page) enableNetwork(ctx context.Context) error {
	return p.conn.call(ctx, "Network.enable", map[string]any{
		"maxTotalBufferSize":    67108864,
		"maxResourceBufferSize": 16777216,
	}, p.sessionID, nil)
}

func (s *captureState) handle(ev cdpEvent) {
	if ev.SessionID != s.sessionID {
		return
	}
	switch ev.Method {
	case "Network.requestWillBeSent":
		s.onRequest(ev.Params)
	case "Network.responseReceived":
		s.onResponse(ev.Params)
	case "Network.loadingFinished":
		s.onFinished(ev.Params)
	case "Network.loadingFailed":
		s.onFailed(ev.Params)
	}
}

// onRequest 重定向复用 requestId,以最后一跳 URL 为准;最后一跳不匹配则丢弃。
func (s *captureState) onRequest(raw json.RawMessage) {
	var params struct {
		RequestID string `json:"requestId"`
		Request   struct {
			URL string `json:"url"`
		} `json:"request"`
	}
	if json.Unmarshal(raw, &params) != nil || params.RequestID == "" {
		return
	}
	u, err := url.Parse(params.Request.URL)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil || !s.match(u) {
		delete(s.inflight, params.RequestID)
		return
	}
	s.inflight[params.RequestID] = u
	s.lastActivity = time.Now()
}

func (s *captureState) onResponse(raw json.RawMessage) {
	var params struct {
		RequestID string `json:"requestId"`
		Response  struct {
			MimeType string `json:"mimeType"`
		} `json:"response"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return
	}
	s.mu.Lock()
	if _, ok := s.inflight[params.RequestID]; ok {
		s.mime[params.RequestID] = params.Response.MimeType
	}
	s.mu.Unlock()
}

func (s *captureState) onFinished(raw json.RawMessage) {
	var params struct {
		RequestID string `json:"requestId"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return
	}
	s.mu.Lock()
	u := s.inflight[params.RequestID]
	delete(s.inflight, params.RequestID)
	mime := s.mime[params.RequestID]
	delete(s.mime, params.RequestID)
	if u != nil {
		s.lastActivity = time.Now()
		// mime 为空说明无从判断,与 TS 一致:仍尝试取 body,只有明确非文本才丢弃。
		if mime == "" || textMIME.MatchString(mime) {
			s.finished = append(s.finished, finishedReq{requestID: params.RequestID, url: u.String()})
		}
	}
	s.mu.Unlock()
}

func (s *captureState) onFailed(raw json.RawMessage) {
	var params struct {
		RequestID string `json:"requestId"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return
	}
	s.mu.Lock()
	if _, ok := s.inflight[params.RequestID]; ok {
		s.lastActivity = time.Now() // 与 TS 一致:匹配请求失败也算活动,给重试留窗口
	}
	delete(s.inflight, params.RequestID)
	delete(s.mime, params.RequestID)
	s.mu.Unlock()
}

func (s *captureState) takeFinished() []finishedReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.finished
	s.finished = nil
	return out
}

// markCommitted 在新文档就绪时重置静默窗口:idle 从导航完成算起,
// 而不是从调用 CaptureNavigate 算起,慢导航不会提前结束捕获。
func (s *captureState) markCommitted() {
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
}

func (s *captureState) idleOK(idle time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inflight) == 0 && time.Since(s.lastActivity) >= idle
}

func (p *page) collectCaptured(ctx context.Context, st *captureState, before any, sameDoc bool, idle, timeout time.Duration) ([]platform.CapturedResponse, error) {
	if idle <= 0 {
		idle = time.Second
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	deadline := time.Now().Add(timeout)
	results := []platform.CapturedResponse{}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	committed := false
	for {
		if p.conn.isClosed() {
			return results, fmt.Errorf("%w: 捕获期间 CDP 连接已关闭", platform.ErrBrowserUnavailable)
		}
		results = p.fetchBodies(ctx, st.takeFinished(), results)
		if !committed {
			ok, err := p.docCommitted(ctx, before, sameDoc)
			if err != nil {
				return results, err
			}
			if ok {
				committed = true
				st.markCommitted()
			}
		}
		if committed && st.idleOK(idle) {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case <-ticker.C:
		}
	}
	results = p.fetchBodies(context.WithoutCancel(ctx), st.takeFinished(), results)
	return results, nil
}

// docCommitted 用 §2.5 的完整守卫:新文档提交(timeOrigin 变化)且 readyState
// complete;same-document 导航没有新文档,只需 complete。
func (p *page) docCommitted(ctx context.Context, before any, sameDoc bool) (bool, error) {
	expr := `document.readyState === 'complete'`
	if f, ok := before.(float64); ok && !sameDoc {
		expr = fmt.Sprintf("performance.timeOrigin !== %s && document.readyState === 'complete'",
			strconv.FormatFloat(f, 'f', -1, 64))
	}
	v, err := p.Evaluate(ctx, expr)
	if err != nil {
		if errors.Is(err, platform.ErrBrowserUnavailable) {
			return false, err
		}
		return false, nil // 导航过渡期 context destroyed = 新文档尚未提交
	}
	b, _ := v.(bool)
	return b, nil
}

func (p *page) fetchBodies(ctx context.Context, reqs []finishedReq, results []platform.CapturedResponse) []platform.CapturedResponse {
	for _, req := range reqs {
		var out struct {
			Body          string `json:"body"`
			Base64Encoded bool   `json:"base64Encoded"`
		}
		err := p.conn.call(ctx, "Network.getResponseBody",
			map[string]any{"requestId": req.requestID}, p.sessionID, &out)
		if err != nil {
			continue
		}
		body := out.Body
		if out.Base64Encoded {
			if raw, err := base64.StdEncoding.DecodeString(out.Body); err == nil {
				body = string(raw)
			}
		}
		if body == "" {
			continue
		}
		results = append(results, platform.CapturedResponse{URL: req.url, Body: body})
	}
	return results
}
