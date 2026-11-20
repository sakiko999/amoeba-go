// page 是 Do 内的页面句柄:所有命令都带 sessionId。Navigate 用
// performance.timeOrigin 守卫避免读到旧文档;Evaluate 固定
// awaitPromise+returnByValue;WaitUntil 轮询吞掉导航过渡错误。

package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"mygo-host/platform"
)

const navTimeout = 20 * time.Second

var errPollTimeout = errors.New("轮询超时")

type page struct {
	conn      *cdpConn
	key       string
	sessionID string
}

func (p *page) Navigate(ctx context.Context, rawURL string) error {
	if err := p.checkURL(rawURL); err != nil {
		return err
	}
	before, _ := p.Evaluate(ctx, "performance.timeOrigin")
	loaderID, err := p.navigate(ctx, rawURL)
	if err != nil {
		return err
	}
	return p.awaitReady(ctx, before, rawURL, loaderID == "")
}

// checkURL 防跨 origin 串页:rawURL 规范化后必须等于 Do 的 key。
func (p *page) checkURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || urlOrigin(u) != p.key {
		return fmt.Errorf("browser: 导航地址与 target origin 不一致(%s)", rawURL)
	}
	return nil
}

// navigate 返回 loaderId;为空表示 same-document 导航(timeOrigin 不会变)。
func (p *page) navigate(ctx context.Context, rawURL string) (string, error) {
	var out struct {
		FrameID   string `json:"frameId"`
		LoaderID  string `json:"loaderId"`
		ErrorText string `json:"errorText"`
	}
	if err := p.conn.call(ctx, "Page.navigate", map[string]any{"url": rawURL}, p.sessionID, &out); err != nil {
		return "", err
	}
	if out.ErrorText != "" {
		return "", fmt.Errorf("browser: 导航失败(%s): %s", rawURL, out.ErrorText)
	}
	return out.LoaderID, nil
}

func (p *page) awaitReady(ctx context.Context, before any, rawURL string, sameDoc bool) error {
	cond := `document.readyState === 'complete'`
	if f, ok := before.(float64); ok && !sameDoc {
		cond = fmt.Sprintf("performance.timeOrigin !== %s && document.readyState === 'complete'",
			strconv.FormatFloat(f, 'f', -1, 64))
	}
	if err := p.poll(ctx, cond, navTimeout); err != nil {
		if errors.Is(err, errPollTimeout) {
			return fmt.Errorf("%w: 页面未加载完成: %s", platform.ErrBrowserTimeout, rawURL)
		}
		return err
	}
	return nil
}

func (p *page) Evaluate(ctx context.Context, expression string) (any, error) {
	var out cdpEvalOut
	err := p.conn.call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
	}, p.sessionID, &out)
	if err != nil {
		return nil, err
	}
	return out.value(expression)
}

type cdpEvalOut struct {
	Result struct {
		Type        string          `json:"type"`
		Subtype     string          `json:"subtype"`
		Value       json.RawMessage `json:"value"`
		Description string          `json:"description"`
	} `json:"result"`
	ExceptionDetails *struct {
		Text      string `json:"text"`
		Exception *struct {
			Description string `json:"description"`
		} `json:"exception"`
	} `json:"exceptionDetails"`
}

// value 按 CDP 嵌套(result.result)解码;异常与不可序列化都转 BrowserEvalError。
func (o *cdpEvalOut) value(expr string) (any, error) {
	if o.ExceptionDetails != nil {
		desc := o.ExceptionDetails.Text
		if e := o.ExceptionDetails.Exception; e != nil && e.Description != "" {
			desc = e.Description
		}
		return nil, &platform.BrowserEvalError{Expression: expr, Description: desc}
	}
	if len(o.Result.Value) == 0 {
		if o.Result.Type == "" || o.Result.Type == "undefined" {
			return nil, nil
		}
		return nil, &platform.BrowserEvalError{
			Expression:  expr,
			Description: "返回值不可 JSON 序列化(" + o.Result.Description + ")",
		}
	}
	var v any
	if err := json.Unmarshal(o.Result.Value, &v); err != nil {
		return nil, &platform.BrowserEvalError{Expression: expr, Description: "返回值解码失败: " + err.Error()}
	}
	return v, nil
}

func (p *page) WaitUntil(ctx context.Context, condition string, timeout time.Duration) error {
	expr := "(() => { try { return (" + condition + ") === true; } catch { return false; } })()"
	if err := p.poll(ctx, expr, timeout); err != nil {
		if errors.Is(err, errPollTimeout) {
			return fmt.Errorf("%w: 等待页面条件超时: %s", platform.ErrBrowserTimeout, condition)
		}
		return err
	}
	return nil
}

// poll 每 250ms 求值一次;导航过渡期错误(Execution context destroyed 等)视为未就绪。
func (p *page) poll(ctx context.Context, expr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if v, err := p.Evaluate(ctx, expr); err == nil {
			if b, ok := v.(bool); ok && b {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return errPollTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Cookies 走 CDP 才读得到 HttpOnly(如 xhs 的 web_session)。
func (p *page) Cookies(ctx context.Context, rawURL string) (map[string]string, error) {
	var out struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	if err := p.conn.call(ctx, "Network.getCookies",
		map[string]any{"urls": []string{rawURL}}, p.sessionID, &out); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(out.Cookies))
	for _, c := range out.Cookies {
		m[c.Name] = c.Value
	}
	return m, nil
}
