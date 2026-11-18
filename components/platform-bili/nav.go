// nav / buvid 共享状态：nav 按 cookie 分 key（不同登录态 mid 不同），失败逐出缓存；
// 匿名 buvid3/4 一次获取（失败同样逐出，避免一次抖动把进程内所有渠道打死）。

package bili

import (
	"context"
	"fmt"
	"regexp"

	"mygo-host/platform"
)

type navEntry struct {
	done chan struct{}
	data navData
	err  error
}

type buvidCall struct {
	done  chan struct{}
	value string
	err   error
}

// nav 一次 nav（带 cookie 取登录态 mid + wbi_img），按 cookie 串缓存。
func (c *client) nav(ctx context.Context, cookie string) (navData, error) {
	c.mu.Lock()
	if e, ok := c.navs[cookie]; ok {
		c.mu.Unlock()
		<-e.done
		return e.data, e.err
	}
	e := &navEntry{done: make(chan struct{})}
	c.navs[cookie] = e
	c.mu.Unlock()

	e.data, e.err = c.fetchNav(ctx, cookie)

	c.mu.Lock()
	if e.err != nil {
		delete(c.navs, cookie)
	}
	close(e.done)
	c.mu.Unlock()
	return e.data, e.err
}

func (c *client) fetchNav(ctx context.Context, cookie string) (navData, error) {
	headers := map[string]string{"user-agent": bilibiliUA, "referer": defaultReferer}
	if cookie != "" {
		headers["cookie"] = cookie
	}
	raw, err := c.sc.HTTP.GetJSON(ctx, apiMain+"/x/web-interface/nav", headers)
	if err != nil {
		return navData{}, err
	}
	var data navData
	if err := platform.UnmarshalJSON(raw, &data); err != nil {
		return navData{}, err
	}
	return data, nil
}

// navMid 登录态 mid（未登录/无 cookie 为 0）。
func (c *client) navMid(ctx context.Context, cookieOpt *string) (int, error) {
	data, err := c.nav(ctx, c.resolveCookie(cookieOpt))
	if err != nil {
		return 0, err
	}
	if data.Data == nil || data.Data.Mid == nil {
		return 0, nil
	}
	return int(*data.Data.Mid), nil
}

// ensureBuvid 匿名 buvid3/4 就绪（与 cookie 无关；空指纹也缓存，与 TS 完成后不再重发一致）。
func (c *client) ensureBuvid(ctx context.Context) error {
	c.mu.Lock()
	if c.buvidSet {
		c.mu.Unlock()
		return nil
	}
	if call := c.buvidCall; call != nil {
		c.mu.Unlock()
		<-call.done
		return call.err
	}
	call := &buvidCall{done: make(chan struct{})}
	c.buvidCall = call
	c.mu.Unlock()

	call.value, call.err = c.fetchBuvid(ctx)

	c.mu.Lock()
	if call.err == nil {
		c.buvid, c.buvidSet = call.value, true
	}
	c.buvidCall = nil
	close(call.done)
	c.mu.Unlock()
	return call.err
}

func (c *client) fetchBuvid(ctx context.Context) (string, error) {
	headers := map[string]string{"user-agent": bilibiliUA, "referer": defaultReferer}
	raw, err := c.sc.HTTP.GetJSON(ctx, apiMain+"/x/frontend/finger/spi", headers)
	if err != nil {
		return "", err
	}
	var resp struct {
		Data *struct {
			B3 *string `json:"b_3"`
			B4 *string `json:"b_4"`
		} `json:"data"`
	}
	if err := platform.UnmarshalJSON(raw, &resp); err != nil {
		return "", err
	}
	b3, b4 := "", ""
	if resp.Data != nil {
		b3, b4 = deref(resp.Data.B3), deref(resp.Data.B4)
	}
	if b3 == "" {
		return "", nil
	}
	return fmt.Sprintf("buvid3=%s;buvid4=%s;", b3, b4), nil
}

func (c *client) anonBuvid() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buvid
}

var buvid3ValueRe = regexp.MustCompile(`buvid3=([^;]+)`)

// anonBuvid3 匿名 buvid3（直播弹幕未登录认证用）。
func (c *client) anonBuvid3(ctx context.Context) (string, error) {
	if err := c.ensureBuvid(ctx); err != nil {
		return "", err
	}
	if m := buvid3ValueRe.FindStringSubmatch(c.anonBuvid()); len(m) == 2 {
		return m[1], nil
	}
	return "", nil
}
