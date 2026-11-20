// 登录态与密钥服务组件：page-credentials 页面的数据面。登录态的唯一权威是
// 载具 profile 的 cookie，但载具按任务启停，所以 Status 读 credentials.json
// 镜像；扫码登录的会话生命周期与镜像同步见 login.go，平台配方由 platform-*
// 组件提供（login.Flow），本组件不含平台私有选择器或协议。

package credentials

import (
	"context"
	_ "embed"
	"errors"
	"sync"

	"mygo-host/platform"
	"mygo-host/platform/login"
)

//go:embed component.json
var manifestRaw []byte

// Service 页面绑定面:经 mygo.BindAs("Credentials", svc) 暴露。
type Service struct {
	creds   *platform.Credentials
	browser login.Browser
	env     *login.Env
	order   []login.Flow
	flows   map[string]login.Flow
	keys    []platform.KeySpec

	mu       sync.Mutex
	sessions map[string]*login.Session
}

// NewService 返回页面绑定用的 Service 与参与拓扑激活的组件包装。
// flows 由宿主从各 platform-* 组件收集；keys 是密钥的展示清单。
func NewService(creds *platform.Credentials, b login.Browser, httpClient *platform.HostClient,
	logf func(string, ...any), flows []login.Flow, keys []platform.KeySpec) (*Service, platform.Service, error) {
	m, err := platform.ParseServiceManifest(manifestRaw)
	if err != nil {
		return nil, nil, err
	}
	if creds == nil || b == nil {
		return nil, nil, errors.New("credentials: 缺少凭据存储或载具桥")
	}
	index := make(map[string]login.Flow, len(flows))
	for _, f := range flows {
		if f.ID == "" {
			return nil, nil, errors.New("credentials: 登录配方缺少平台 id")
		}
		index[f.ID] = f
	}
	s := &Service{
		creds: creds, browser: b, order: flows, flows: index, keys: keys,
		env:      &login.Env{HTTP: httpClient, Browser: b, Logf: logf},
		sessions: map[string]*login.Session{},
	}
	return s, &component{s: s, m: m}, nil
}

type component struct {
	s *Service
	m platform.ServiceManifest
}

func (c *component) Manifest() platform.ServiceManifest { return c.m }

func (c *component) Activate(platform.Registrar) error { return nil }

// PlatformStatus 是页面上的一张平台卡片。
type PlatformStatus struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Color    string `json:"color,omitempty"`
	LoggedIn bool   `json:"loggedIn"`
	User     string `json:"user,omitempty"`
	Stored   bool   `json:"stored"` // credentials.json 里有 cookie 镜像
}

type StatusResult struct {
	Platforms []PlatformStatus `json:"platforms"`
}

// Status 从 credentials.json 镜像计算登录态:载具按任务启停,不在轮询路径上
// 启动任何进程;镜像在登录成功/退出登录时更新。
func (s *Service) Status(context.Context) (StatusResult, error) {
	res := StatusResult{Platforms: []PlatformStatus{}}
	for _, pf := range s.order {
		raw, stored := s.creds.Lookup(pf.ID)
		cookies := login.ParseCookiePairs(raw)
		item := PlatformStatus{ID: pf.ID, Label: pf.Label, Color: pf.Color, Stored: stored}
		if stored {
			item.LoggedIn = pf.LoggedIn(cookies)
			if item.LoggedIn {
				item.User = pf.User(cookies) // 未登录状态的残留标识不算账号
			}
		}
		res.Platforms = append(res.Platforms, item)
	}
	return res, nil
}

type PlatformParams struct {
	Platform string `json:"platform"`
}

type LogoutResult struct {
	Cleared int `json:"cleared"`
}

// Logout 清浏览器 profile 里该平台域的 cookie(含 HttpOnly),再清本地镜像。
func (s *Service) Logout(ctx context.Context, p PlatformParams) (LogoutResult, error) {
	pf, err := s.flow(p.Platform)
	if err != nil {
		return LogoutResult{}, err
	}
	n, err := s.browser.ClearCookies(ctx, pf.Hosts)
	if err != nil {
		return LogoutResult{}, err
	}
	if err := s.creds.Set(pf.ID, ""); err != nil {
		return LogoutResult{}, err
	}
	s.dropSession(pf.ID)
	s.closeLoginTab(ctx, pf.ID)
	return LogoutResult{Cleared: n}, nil
}

func (s *Service) flow(id string) (login.Flow, error) {
	pf, ok := s.flows[id]
	if !ok {
		return login.Flow{}, errors.New("未知平台: " + id)
	}
	return pf, nil
}

// mirrorCookies 读 credentials.json 镜像(不碰载具)。
func (s *Service) mirrorCookies(id string) map[string]string {
	raw, _ := s.creds.Lookup(id)
	return login.ParseCookiePairs(raw)
}

// syncCreds 把浏览器 cookie 镜像到 credentials.json(空则不写)。
func (s *Service) syncCreds(id string, cookies map[string]string) {
	if len(cookies) == 0 {
		return
	}
	if err := s.creds.Set(id, login.SerializeCookies(cookies)); err != nil {
		s.env.Log("credentials: 同步 %s cookie 失败: %v", id, err)
	}
}

func (s *Service) session(id string) *login.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func (s *Service) setSession(sess *login.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.Platform] = sess
}

func (s *Service) dropSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// dropSessionIf 只在会话仍是当前会话时删除;刷新已换上新会话时,旧轮询
// 不得再关闭新 target。
func (s *Service) dropSessionIf(id string, sess *login.Session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[id] != sess {
		return false
	}
	delete(s.sessions, id)
	return true
}
