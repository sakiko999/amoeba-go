// browser 载具的生产端:按需拉起一个独立的 mygo/WebView2 子进程,经
// WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=0 开原生
// CDP,消费者直接连 127.0.0.1 上的标准端点(不再自建代理)。
//
// 为什么必须是子进程:该环境变量是 WebView2 环境级开关,设在宿主进程上会
// 把宿主自己的受信页面(含 window.mygo 桥)一起暴露给本机任意进程。子进程
// 有独立 profile 与独立 WebView2 环境,宿主页面不受影响;载具自身也只用
// about:blank(Untrusted,无桥)。持久化数据在 profile/EBWebView(cookie、
// localStorage、IndexedDB、Cache、Service Worker),由 WEBVIEW2_USER_DATA_FOLDER
// 显式指向,见 carrierEnv。

package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const carrierFlag = "--browser-carrier"

// Endpoint 载具的 CDP 端点(HTTP /json/* 与 browser WebSocket)。
type Endpoint struct {
	Port int    `json:"port"`
	HTTP string `json:"http"`
	WS   string `json:"ws"`
}

// endpointSnapshot Endpoint 的内部形态:gen 用于识别进程重启(端口每次都会变)。
type endpointSnapshot struct {
	Endpoint
	gen uint64
}

// Status 载具进程状态(诊断用;不触发启动)。
// UserDataDir 是传给 WEBVIEW2_USER_DATA_FOLDER 的根;真正的 Chromium
// profile 是它的 EBWebView 子目录,即 ProfileDir。
type Status struct {
	Running     bool   `json:"running"`
	PID         int    `json:"pid,omitempty"`
	Port        int    `json:"port,omitempty"`
	UserDataDir string `json:"userDataDir"`
	ProfileDir  string `json:"profileDir"`
	Detail      string `json:"detail,omitempty"`
}

// Service 载具的懒启动管理器:首个 endpoint 调用才起子进程。
type Service struct {
	profile string
	logf    func(string, ...any)

	mu    sync.Mutex
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	port  int
	wsURL string
	gen   uint64 // 每次 startLocked 成功后自增;连接层据此识别旧会话
	job   *jobHandle

	// childCommand 默认重执行同一二进制;测试注入测试进程本身(见 carrier_test.go)。
	childCommand func() (*exec.Cmd, error)
}

func newService(profileDir string, logf func(string, ...any)) *Service {
	s := &Service{profile: profileDir, logf: logf}
	s.childCommand = s.defaultChildCommand
	return s
}

func (s *Service) defaultChildCommand() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return exec.Command(exe, carrierFlag), nil
}

// Endpoint 确保载具在运行并返回当前 CDP 端点(重启后端口会变)。
func (s *Service) Endpoint(ctx context.Context) (Endpoint, error) {
	snap, err := s.snapshot(ctx)
	return snap.Endpoint, err
}

// snapshot 供桥接层在连接断开时重新取端点与代次;不要在平台层缓存。
func (s *Service) snapshot(ctx context.Context) (endpointSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLocked(ctx); err != nil {
		return endpointSnapshot{}, err
	}
	return endpointSnapshot{
		Endpoint: Endpoint{Port: s.port, HTTP: httpBase(s.port), WS: s.wsURL},
		gen:      s.gen,
	}, nil
}

// TargetCount 统计 /json/list 里的 page target(含载具自带的基础页)。
func (s *Service) TargetCount(ctx context.Context) (int, error) {
	ep, err := s.Endpoint(ctx)
	if err != nil {
		return 0, err
	}
	var list []struct {
		Type string `json:"type"`
	}
	if err := getJSON(ctx, ep.HTTP+"/json/list", &list); err != nil {
		return 0, err
	}
	count := 0
	for _, target := range list {
		if target.Type == "page" {
			count++
		}
	}
	return count, nil
}

// Status 只读进程状态,不启动。
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{
		UserDataDir: s.profile,
		ProfileDir:  filepath.Join(s.profile, "EBWebView"),
	}
	if s.cmd != nil && !channelClosed(s.done) {
		status.Running = true
		status.PID = s.cmd.Process.Pid
		status.Port = s.port
		status.Detail = s.wsURL
	}
	return status
}

// runningNow 非阻塞地报告进程是否在运行;启动/停止期间拿不到锁时返回 false,
// 让 CloseTab 的补关直接放弃而不是等一个可能长达 20s 的冷启动。
func (s *Service) runningNow() bool {
	if !s.mu.TryLock() {
		return false
	}
	defer s.mu.Unlock()
	return s.cmd != nil && !channelClosed(s.done)
}

// Tidy 请求载具立即整理一次窗口(去任务栏)。best-effort:载具未运行时忽略;
// 与 Stop 竞态时写管道会失败,同样忽略。
func (s *Service) Tidy() {
	s.mu.Lock()
	stdin := s.stdin
	s.mu.Unlock()
	if stdin != nil {
		_, _ = io.WriteString(stdin, "tidy\n")
	}
}

// Stop 停子进程并保留 profile(下次 endpoint 会重新拉起)。
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

// Reset 停子进程并删除整个 profile(登录态、缓存全部清除)。
func (s *Service) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopLocked(); err != nil {
		return err
	}
	var err error
	for range 20 {
		if err = os.RemoveAll(s.profile); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("清理载具 profile 失败: %w", err)
}

func (s *Service) ensureLocked(ctx context.Context) error {
	if s.cmd != nil && !channelClosed(s.done) && s.port > 0 && s.wsURL != "" {
		if versionOK(ctx, s.port) {
			return nil
		}
	}
	if s.cmd != nil {
		_ = s.stopLocked()
	}
	return s.startLocked()
}

func (s *Service) startLocked() error {
	if runtime.GOOS != "windows" {
		return errors.New("browser 载具仅支持 Windows(WebView2)")
	}
	if s.profile == "" {
		return errors.New("browser 载具缺少 profile 目录")
	}
	if err := os.MkdirAll(s.profile, 0o755); err != nil {
		return err
	}
	cmd, err := s.childCommand()
	if err != nil {
		return err
	}
	stdin, stdinW, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	cmd.Stdout = logWriter{s.logf}
	cmd.Stderr = logWriter{s.logf}
	cmd.Env = carrierEnv(cmd.Env, s.profile)
	// 残留的 DevToolsActivePort 是上次进程的端口;先删掉,waitPort 只认新进程写出的文件。
	_ = os.Remove(filepath.Join(s.profile, "EBWebView", "DevToolsActivePort"))
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdinW.Close()
		return fmt.Errorf("启动载具子进程失败: %w", err)
	}
	_ = stdin.Close()
	job := newJob()
	if err := job.assign(cmd); err != nil && s.logf != nil {
		s.logf("browser carrier: JobObject 兜底未生效: %v", err)
	}

	done := make(chan struct{})
	s.cmd, s.stdin, s.done, s.job = cmd, stdinW, done, job
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	port, err := s.waitPort(ctx)
	if err != nil {
		_ = s.stopLocked()
		return err
	}
	wsURL, err := waitBrowserWS(ctx, port)
	if err != nil {
		_ = s.stopLocked()
		return err
	}
	s.port, s.wsURL = port, wsURL
	s.gen++
	if s.logf != nil {
		s.logf("browser carrier ready: 127.0.0.1:%d", port)
	}
	return nil
}

// waitPort 读 WebView2 写在 profile/EBWebView/DevToolsActivePort 的随机端口。
func (s *Service) waitPort(ctx context.Context) (int, error) {
	path := filepath.Join(s.profile, "EBWebView", "DevToolsActivePort")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if port, err := readActivePort(path); err == nil && port > 0 {
			return port, nil
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("等待 CDP 端口超时(%s)", path)
		case <-s.done:
			return 0, errors.New("载具子进程提前退出")
		case <-ticker.C:
		}
	}
}

func (s *Service) stopLocked() error {
	if s.cmd == nil {
		return nil
	}
	if s.stdin != nil {
		_ = s.stdin.Close() // 子进程读 stdin EOF 后 App.Quit
		s.stdin = nil
	}
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
		}
	}
	// job 关闭时内核回收整棵 WebView2 子树;正常退出时也同样兜底。
	s.job.close()
	s.cmd, s.stdin, s.done, s.port, s.wsURL, s.job = nil, nil, nil, 0, "", nil
	return nil
}

func channelClosed(ch chan struct{}) bool {
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
