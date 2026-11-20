// 扫码登录的会话编排：取码、轮询、刷新复用 pin、取消/过期收尾与 cookie 镜像。
// 平台差异全部封装在 login.Flow 里；这里只按「HTTP 优先 → 浏览器」的顺序调用。

package credentials

import (
	"context"
	"errors"
	"time"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

type StartResult struct {
	State string `json:"state"` // qr | already
	QR    string `json:"qr,omitempty"`
	// QRText 二维码内容(纯 API 平台只给内容,B 站);页面本地编码成图片。
	QRText    string `json:"qrText,omitempty"`
	User      string `json:"user,omitempty"`
	ExpiresIn int    `json:"expiresIn,omitempty"` // 秒
}

type PollResult struct {
	State   string `json:"state"` // pending | success | expired
	User    string `json:"user,omitempty"`
	Message string `json:"message,omitempty"`
}

// LoginStart 打开该平台的登录页并取二维码;已有可信登录态时直接返回 already。
func (s *Service) LoginStart(ctx context.Context, p PlatformParams) (StartResult, error) {
	pf, err := s.flow(p.Platform)
	if err != nil {
		return StartResult{}, err
	}
	// 刷新会再次进入 LoginStart:先摘掉旧会话,已在途的旧轮询便不会再关闭
	// 这次要复用的 target。登录 target 与实例 pin 保留,避免重新冷启动。
	s.dropSession(pf.ID)
	cookies, err := s.currentCookies(ctx, pf)
	if err != nil {
		return StartResult{}, err
	}
	if pf.TrustCookies && pf.LoggedIn(cookies) {
		s.syncCreds(pf.ID, cookies)
		return StartResult{State: "already", User: pf.User(cookies)}, nil
	}

	sess := &login.Session{
		Platform: pf.ID, BaselineUserID: cookies[pf.BaselineCookie],
		Deadline: time.Now().Add(pf.Timeout),
	}
	if res, handled, err := s.startHTTP(ctx, pf, sess); handled {
		return res, err
	}
	return s.startBrowser(ctx, pf, sess)
}

// LoginPoll 轮询扫码结果;成功后把 cookie 串同步进 credentials.json。
func (s *Service) LoginPoll(ctx context.Context, p PlatformParams) (PollResult, error) {
	pf, err := s.flow(p.Platform)
	if err != nil {
		return PollResult{}, err
	}
	sess := s.session(pf.ID)
	if sess == nil {
		return s.recoverLostSession(ctx, pf)
	}
	if pf.HTTPPoll != nil && sess.Data != nil {
		if res, done, err := s.pollHTTP(ctx, pf, sess); done {
			return res, err
		}
	} else {
		if res, done, err := s.pollBrowser(ctx, pf, sess); done {
			return res, err
		}
	}
	if time.Now().After(sess.Deadline) {
		if s.dropSessionIf(pf.ID, sess) && sess.UsedBrowser {
			s.closeLoginTab(ctx, pf.ID) // 释放 Hold 住的平台实例
		}
		return PollResult{State: "expired", Message: "二维码已过期,请点击刷新"}, nil
	}
	return PollResult{State: "pending"}, nil
}

// LoginCancel 取消扫码并关闭专属登录 target。
func (s *Service) LoginCancel(ctx context.Context, p PlatformParams) error {
	pf, err := s.flow(p.Platform)
	if err != nil {
		return err
	}
	sess := s.session(pf.ID)
	s.dropSession(pf.ID)
	// 纯 HTTP 会话没有 login target,不必为关一个不存在的页面拉起载具。
	if sess == nil || sess.UsedBrowser {
		s.closeLoginTab(ctx, pf.ID)
	}
	return nil
}

// currentCookies 纯 HTTP 平台看镜像;浏览器平台优先复用 Hold 住的实例,
// 避免刷新时重新 Acquire 被 pin 住的 profile。
func (s *Service) currentCookies(ctx context.Context, pf login.Flow) (map[string]string, error) {
	if pf.Open == nil || pf.HTTPStart != nil {
		return s.mirrorCookies(pf.ID), nil
	}
	all, err := s.browser.CookiesAs(ctx, login.Key(pf.ID), pf.Origin)
	if err != nil {
		return nil, err
	}
	return login.CookiesFor(all, pf.Hosts), nil
}

// startHTTP 先走纯 HTTP 取码;失败且平台有浏览器实现时回落浏览器。
func (s *Service) startHTTP(ctx context.Context, pf login.Flow, sess *login.Session) (StartResult, bool, error) {
	if pf.HTTPStart == nil {
		return StartResult{}, false, nil
	}
	qr, qrText, err := pf.HTTPStart(ctx, s.env, sess)
	if err == nil && (qr != "" || qrText != "") {
		s.setSession(sess)
		return StartResult{State: "qr", QR: qr, QRText: qrText, ExpiresIn: int(pf.Timeout.Seconds())}, true, nil
	}
	if pf.Open == nil {
		if err == nil {
			err = errors.New("二维码未出现(平台返回空内容)")
		}
		return StartResult{}, true, err
	}
	if err != nil {
		s.env.Log("credentials: %s HTTP 取码失败,回退浏览器: %v", pf.ID, err)
	}
	sess.Data = nil
	return StartResult{}, false, nil
}

func (s *Service) startBrowser(ctx context.Context, pf login.Flow, sess *login.Session) (StartResult, error) {
	key := login.Key(pf.ID)
	sess.UsedBrowser = true
	// 登录页要在扫码期间存活:先 Hold 住平台实例,DoAs 复用该实例建 target。
	if err := s.browser.Hold(ctx, key, pf.Origin); err != nil {
		return StartResult{}, err
	}
	var qr string
	var already bool
	err := s.browser.DoAs(ctx, key, pf.Origin, func(bp platform.BrowserPage) error {
		var e error
		qr, already, e = pf.Open(ctx, s.env, bp)
		return e
	})
	if err != nil {
		s.closeLoginTab(ctx, pf.ID)
		return StartResult{}, err
	}
	if already {
		all, err := s.browser.CookiesAs(ctx, key, pf.Origin)
		if err != nil {
			s.closeLoginTab(ctx, pf.ID)
			return StartResult{}, err
		}
		cookies := login.CookiesFor(all, pf.Hosts)
		s.syncCreds(pf.ID, cookies)
		s.closeLoginTab(ctx, pf.ID)
		return StartResult{State: "already", User: pf.User(cookies)}, nil
	}
	if qr == "" {
		s.closeLoginTab(ctx, pf.ID)
		return StartResult{}, errors.New("二维码未出现(登录页结构可能已变)")
	}
	s.setSession(sess)
	return StartResult{State: "qr", QR: qr, ExpiresIn: int(pf.Timeout.Seconds())}, nil
}

// recoverLostSession 服务重启/页面刷新后会话丢失:cookie 已经是登录态就直接补同步。
func (s *Service) recoverLostSession(ctx context.Context, pf login.Flow) (PollResult, error) {
	cookies, err := s.currentCookies(ctx, pf)
	if err != nil {
		return PollResult{}, err
	}
	if pf.LoggedIn(cookies) {
		s.syncCreds(pf.ID, cookies)
		s.closeLoginTab(ctx, pf.ID)
		return PollResult{State: "success", User: pf.User(cookies)}, nil
	}
	return PollResult{State: "expired", Message: "登录会话已失效,请重新扫码"}, nil
}

func (s *Service) pollHTTP(ctx context.Context, pf login.Flow, sess *login.Session) (PollResult, bool, error) {
	done, cookies, user, err := pf.HTTPPoll(ctx, s.env, sess)
	if errors.Is(err, login.ErrExpired) {
		s.dropSessionIf(pf.ID, sess)
		return PollResult{State: "expired", Message: err.Error()}, true, nil
	}
	if err != nil {
		return PollResult{}, true, err
	}
	if !done {
		return PollResult{}, false, nil
	}
	s.syncCreds(pf.ID, cookies)
	s.dropSessionIf(pf.ID, sess)
	return PollResult{State: "success", User: user}, true, nil
}

func (s *Service) pollBrowser(ctx context.Context, pf login.Flow, sess *login.Session) (PollResult, bool, error) {
	all, err := s.browser.CookiesAs(ctx, login.Key(pf.ID), pf.Origin)
	if err != nil {
		return PollResult{}, true, err
	}
	cookies := login.CookiesFor(all, pf.Hosts)
	done, user := pf.Poll(ctx, s.env, sess, cookies)
	if !done {
		return PollResult{}, false, nil
	}
	s.syncCreds(pf.ID, cookies)
	if s.dropSessionIf(pf.ID, sess) {
		s.closeLoginTab(ctx, pf.ID)
	}
	return PollResult{State: "success", User: user}, true, nil
}

// closeLoginTab 同步关闭登录 target(不继承调用方取消,避免取消路径关不掉窗口)。
// 同步而不是异步:紧接着重新扫码时会复用同一个 key,异步关闭可能误杀新 target。
func (s *Service) closeLoginTab(ctx context.Context, id string) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.browser.CloseTabAs(cctx, login.Key(id)); err != nil {
		s.env.Log("credentials: 关闭 %s 登录 target 失败: %v", id, err)
	}
}
