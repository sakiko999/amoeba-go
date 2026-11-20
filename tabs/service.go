// TabsService 是暴露给 chrome 页面的绑定服务：window.mygo.call("Tabs.*")。
// 方法名转驼峰后即页面侧 API（Kinds→kinds、ToggleTheme→toggleTheme）。
// Manager 在 Bind 之前构造注入：mygo.Bind 会校验全部导出方法的参数类型必须是
// JSON 可编码的，所以管理器只能经构造函数传入，不能作为导出方法参数。
package tabs

import (
	"context"
	"errors"
	"net/url"
	"runtime"

	"github.com/egoist/mygo"
)

var errExternalURL = errors.New("Tabs.OpenExternal 仅支持 https URL")

type Service struct{ mgr *Manager }

func NewService(mgr *Manager) *Service { return &Service{mgr: mgr} }

// SystemInfo 是 sysinfo 页面经 window.tab.systemInfo 取到的运行环境。
// 宿主换成了 mygo，Electron/Chromium/Node 三位留 "—"（页面表格的兼容形状）。
type SystemInfo struct {
	Platform string  `json:"platform"`
	Arch     string  `json:"arch"`
	CPUs     int     `json:"cpus"`
	MemoryGB float64 `json:"memoryGb"`
	Electron string  `json:"electron"`
	Chrome   string  `json:"chrome"`
	Node     string  `json:"node"`
}

func (s *Service) SystemInfo(ctx context.Context) (SystemInfo, error) {
	return SystemInfo{Platform: runtime.GOOS, Arch: runtime.GOARCH, CPUs: runtime.NumCPU(), Electron: "—", Chrome: "—", Node: "—"}, nil
}

func (s *Service) Chrome(ctx context.Context) (ChromeInfo, error) { return s.mgr.ChromeInfo(), nil }

func (s *Service) SetTitle(ctx context.Context, instanceID, title string) error {
	s.mgr.SetTitle(instanceID, title)
	return nil
}

func (s *Service) Ping(ctx context.Context, instanceID, value string) (string, error) {
	return s.mgr.Ping(instanceID, value)
}

func (s *Service) Kinds(ctx context.Context) ([]Kind, error) { return s.mgr.Kinds(), nil }

func (s *Service) Create(ctx context.Context, kind string) error { return s.mgr.Create(kind) }

func (s *Service) Activate(ctx context.Context, instanceID string) error {
	s.mgr.Activate(instanceID)
	return nil
}

func (s *Service) Close(ctx context.Context, instanceID string) error {
	s.mgr.Close(instanceID)
	return nil
}

func (s *Service) Move(ctx context.Context, instanceID, overInstanceID string) error {
	s.mgr.Move(instanceID, overInstanceID)
	return nil
}

// PopupNewTabMenu 对应 chrome 的 “+” 按钮：弹出宿主原生菜单并新建页面。
func (s *Service) PopupNewTabMenu(ctx context.Context) error {
	s.mgr.PopupNewTabMenu()
	return nil
}

// PopupTabMenu 对应 tab 的右键：弹出宿主原生菜单（关闭/关闭其他/关闭右侧）。
func (s *Service) PopupTabMenu(ctx context.Context, instanceID string) error {
	s.mgr.PopupTabMenu(instanceID)
	return nil
}

func (s *Service) State(ctx context.Context) (ThemeInfo, error) { return s.mgr.ThemeInfo(), nil }

func (s *Service) ToggleTheme(ctx context.Context) error {
	s.mgr.ToggleTheme()
	return nil
}

func (s *Service) OpenExternal(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errExternalURL
	}
	return mygo.Shell.OpenExternal(parsed.String())
}
